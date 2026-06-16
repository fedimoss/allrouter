package service

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

type ProviderMail struct {
	Enabled        bool   `json:"enabled"`
	Host           string `json:"host"`
	Port           int    `json:"port"`
	Username       string `json:"username"`
	Password       string `json:"password"`
	FromEmail      string `json:"from_email"`
	FromName       string `json:"from_name"`
	ReplyTo        string `json:"reply_to"`
	Encryption     string `json:"encryption"` // starttls / ssl / none
	ForceAuthLogin bool   `json:"force_auth_login"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}

// 邮件发送
func SendProviderMail(providerId int, subject string, receiver string, content string) error {
	//获取服务商配置
	if providerId != 0 {
		mailConfig, err := model.GetProviderOptionValue(providerId, "mail.smtp")
		if err != nil {
			return err
		}
		if mailConfig == "" {
			return errors.New("该服务商尚未配置邮件服务")
		}
		opt := ProviderMail{}
		err = common.UnmarshalJsonStr(mailConfig, &opt)
		if err != nil {
			return err
		}
		//判断是否启用
		if !opt.Enabled {
			return errors.New("服务商未开启邮件服务")
		}
		cfg := common.SMTPConfig{
			Server:         opt.Host,
			Port:           opt.Port,
			Account:        opt.Username,
			Token:          opt.Password,
			From:           opt.FromEmail,
			FromName:       opt.FromName,
			ReplyTo:        opt.ReplyTo,
			SSLEnabled:     opt.Encryption == "ssl",
			ForceAuthLogin: opt.ForceAuthLogin,
		}
		return common.SendMailBySMTP(cfg, subject, receiver, content)
	}

	return common.SendEmail(subject, receiver, content)
}
