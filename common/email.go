package common

import (
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"net/smtp"
	"slices"
	"strings"
	"time"
)

type SMTPConfig struct {
	Server         string
	Port           int
	Account        string
	Token          string
	From           string
	FromName       string
	ReplyTo        string
	SSLEnabled     bool
	ForceAuthLogin bool
}

func generateMessageID(from string) (string, error) {
	split := strings.Split(from, "@")
	if len(split) < 2 {
		return "", fmt.Errorf("invalid SMTP from")
	}
	domain := split[1]
	return fmt.Sprintf("<%d.%s@%s>", time.Now().UnixNano(), GetRandomString(12), domain), nil
}

func shouldUseSMTPLoginAuth(cfg SMTPConfig) bool {
	if cfg.ForceAuthLogin {
		return true
	}
	return isOutlookServer(cfg.Account) || slices.Contains(EmailLoginAuthServerList, cfg.Server)
}

func getSMTPAuth(cfg SMTPConfig) smtp.Auth {
	if shouldUseSMTPLoginAuth(cfg) {
		return LoginAuth(cfg.Account, cfg.Token)
	}
	return smtp.PlainAuth("", cfg.Account, cfg.Token, cfg.Server)
}

func SendEmail(subject string, receiver string, content string) error {
	cfg := SMTPConfig{
		Server:         SMTPServer,
		Port:           SMTPPort,
		Account:        SMTPAccount,
		Token:          SMTPToken,
		From:           SMTPFrom,
		FromName:       SystemName,
		SSLEnabled:     SMTPSSLEnabled,
		ForceAuthLogin: SMTPForceAuthLogin,
	}
	if cfg.From == "" {
		cfg.From = cfg.Account
	}
	//兼容旧逻辑
	return SendMailBySMTP(cfg, subject, receiver, content)
}

func SendMailBySMTP(cfg SMTPConfig, subject string, receiver string, content string) error {

	if cfg.From == "" {
		return errors.New("SMTP From is empty")
	}
	if cfg.FromName == "" {
		cfg.FromName = SystemName
	}
	id, err2 := generateMessageID(cfg.From)
	if err2 != nil {
		return err2
	}
	if cfg.Server == "" && cfg.Account == "" {
		return fmt.Errorf("SMTP 服务器未配置")
	}
	encodedSubject := fmt.Sprintf("=?UTF-8?B?%s?=", base64.StdEncoding.EncodeToString([]byte(subject)))
	replyToHeader := ""
	if cfg.ReplyTo != "" {
		replyToHeader = fmt.Sprintf("Reply-To: %s\r\n", cfg.ReplyTo)
	}
	mail := []byte(fmt.Sprintf("To: %s\r\n"+
		"From: %s <%s>\r\n"+
		"%s"+
		"Subject: %s\r\n"+
		"Date: %s\r\n"+
		"Message-ID: %s\r\n"+ // 添加 Message-ID 头
		"Content-Type: text/html; charset=UTF-8\r\n\r\n%s\r\n",
		receiver, cfg.FromName, cfg.From, replyToHeader, encodedSubject, time.Now().Format(time.RFC1123Z), id, content))
	auth := getSMTPAuth(cfg)
	addr := fmt.Sprintf("%s:%d", cfg.Server, cfg.Port)
	to := strings.Split(receiver, ";")
	var err error
	if cfg.Port == 465 || cfg.SSLEnabled {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: true,
			ServerName:         cfg.Server,
		}
		conn, err := tls.Dial("tcp", fmt.Sprintf("%s:%d", cfg.Server, cfg.Port), tlsConfig)
		if err != nil {
			return err
		}
		client, err := smtp.NewClient(conn, cfg.Server)
		if err != nil {
			return err
		}
		defer client.Close()
		if err = client.Auth(auth); err != nil {
			return err
		}
		if err = client.Mail(cfg.From); err != nil {
			return err
		}
		receiverEmails := strings.Split(receiver, ";")
		for _, receiver := range receiverEmails {
			if err = client.Rcpt(receiver); err != nil {
				return err
			}
		}
		w, err := client.Data()
		if err != nil {
			return err
		}
		_, err = w.Write(mail)
		if err != nil {
			return err
		}
		err = w.Close()
		if err != nil {
			return err
		}
	} else {
		err = smtp.SendMail(addr, auth, cfg.From, to, mail)
	}
	if err != nil {
		SysError(fmt.Sprintf("failed to send email to %s: %v", receiver, err))
	}
	return err
}
