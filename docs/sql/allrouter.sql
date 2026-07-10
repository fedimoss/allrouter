--
-- PostgreSQL database dump
--

\restrict FIqAjmpw4zcALtpViQTfH3JhEOyV1EoM9Of3mq2OLzJR4Vmvp2YCaeAsTZjY4Nq

-- Dumped from database version 18.3 (Debian 18.3-1.pgdg13+1)
-- Dumped by pg_dump version 18.3 (Debian 18.3-1.pgdg13+1)

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET transaction_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;


CREATE SCHEMA ag_catalog;


CREATE SCHEMA paradedb;


CREATE EXTENSION IF NOT EXISTS pg_search WITH SCHEMA paradedb;


COMMENT ON EXTENSION pg_search IS 'pg_search: Full text search for PostgreSQL using BM25';


CREATE EXTENSION IF NOT EXISTS pg_ivm WITH SCHEMA pg_catalog;


COMMENT ON EXTENSION pg_ivm IS 'incremental view maintenance on PostgreSQL';


CREATE SCHEMA tiger;


CREATE SCHEMA topology;


COMMENT ON SCHEMA topology IS 'PostGIS Topology schema';


CREATE EXTENSION IF NOT EXISTS age WITH SCHEMA ag_catalog;


COMMENT ON EXTENSION age IS 'AGE database extension';


CREATE EXTENSION IF NOT EXISTS fuzzystrmatch WITH SCHEMA public;


COMMENT ON EXTENSION fuzzystrmatch IS 'determine similarities and distance between strings';


CREATE EXTENSION IF NOT EXISTS postgis WITH SCHEMA public;


COMMENT ON EXTENSION postgis IS 'PostGIS geometry and geography spatial types and functions';


CREATE EXTENSION IF NOT EXISTS postgis_tiger_geocoder WITH SCHEMA tiger;


COMMENT ON EXTENSION postgis_tiger_geocoder IS 'PostGIS tiger geocoder and reverse geocoder';


CREATE EXTENSION IF NOT EXISTS postgis_topology WITH SCHEMA topology;


COMMENT ON EXTENSION postgis_topology IS 'PostGIS topology spatial types and functions';


CREATE EXTENSION IF NOT EXISTS vector WITH SCHEMA public;


COMMENT ON EXTENSION vector IS 'vector data type and ivfflat and hnsw access methods';


SET default_tablespace = '';

SET default_table_access_method = heap;


--设置回默认模式 public

SET search_path TO public;


CREATE TABLE abilities (
    "group" character varying(64) NOT NULL,
    model character varying(255) NOT NULL,
    channel_id bigint NOT NULL,
    enabled boolean,
    priority bigint DEFAULT 0,
    weight bigint DEFAULT 0,
    tag text
);



CREATE TABLE admin_sessions (
    id bigint NOT NULL,
    token character varying(64) NOT NULL,
    user_id bigint NOT NULL,
    ip character varying(45) DEFAULT ''::character varying NOT NULL,
    user_agent character varying(512) DEFAULT ''::character varying NOT NULL,
    created_at timestamp with time zone,
    expires_at timestamp with time zone NOT NULL
);


CREATE SEQUENCE admin_sessions_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE admin_sessions_id_seq OWNED BY admin_sessions.id;


CREATE TABLE admin_users (
    id bigint NOT NULL,
    username character varying(64) NOT NULL,
    password_hash character varying(128) NOT NULL,
    password_changed boolean DEFAULT false NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone
);


CREATE SEQUENCE admin_users_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE admin_users_id_seq OWNED BY admin_users.id;


CREATE TABLE channels (
    id bigint NOT NULL,
    type bigint DEFAULT 0,
    key text NOT NULL,
    open_ai_organization text,
    test_model text,
    status bigint DEFAULT 1,
    name text,
    weight bigint DEFAULT 0,
    created_time bigint,
    test_time bigint,
    response_time bigint,
    base_url text DEFAULT ''::text,
    other text,
    balance numeric,
    balance_updated_time bigint,
    models text,
    "group" character varying(64) DEFAULT 'default'::character varying,
    used_quota bigint DEFAULT 0,
    model_mapping text,
    status_code_mapping character varying(1024) DEFAULT ''::character varying,
    priority bigint DEFAULT 0,
    auto_ban bigint DEFAULT 1,
    other_info text,
    tag text,
    setting text,
    param_override text,
    header_override text,
    remark character varying(255),
    channel_info json,
    settings text
);


CREATE SEQUENCE channels_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE channels_id_seq OWNED BY channels.id;


CREATE TABLE checkins (
    id bigint NOT NULL,
    user_id bigint NOT NULL,
    checkin_date character varying(10) NOT NULL,
    quota_awarded bigint NOT NULL,
    created_at bigint,
    provider_id bigint DEFAULT 0 NOT NULL
);


COMMENT ON COLUMN checkins.provider_id IS '所属服务商 ID，0 表示主站签到记录';


CREATE SEQUENCE checkins_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE checkins_id_seq OWNED BY checkins.id;


CREATE TABLE cli_oauth (
    id character varying(50) NOT NULL,
    oauth text NOT NULL,
    model_type integer NOT NULL,
    created_at timestamp(6) with time zone,
    updated_at timestamp(6) with time zone,
    status bigint,
    account_id character varying(250),
    error_reason text
);


COMMENT ON COLUMN cli_oauth.id IS 'ID';



COMMENT ON COLUMN cli_oauth.oauth IS 'OAuth 凭证';


COMMENT ON COLUMN cli_oauth.model_type IS '1: Codex 2: Anthropic 3: Qwen';


COMMENT ON COLUMN cli_oauth.created_at IS '创建时间';


COMMENT ON COLUMN cli_oauth.updated_at IS '更新时间';


COMMENT ON COLUMN cli_oauth.status IS '状态 (1:正常 2:禁用)';


COMMENT ON COLUMN cli_oauth.account_id IS '账户ID';


CREATE TABLE cli_user (
    id character varying(50) NOT NULL,
    status bigint DEFAULT 1,
    user_id character varying(50),
    created_at timestamp(6) with time zone,
    updated_at timestamp(6) with time zone
);

COMMENT ON TABLE cli_user IS 'CLI 用户表';


--
-- Name: COLUMN cli_user.id; Type: COMMENT;;
--

COMMENT ON COLUMN cli_user.id IS 'ID';


--
-- Name: COLUMN cli_user.status; Type: COMMENT;;
--

COMMENT ON COLUMN cli_user.status IS '状态 (1:正常 2:禁用 3:删除)';


--
-- Name: COLUMN cli_user.user_id; Type: COMMENT;;
--

COMMENT ON COLUMN cli_user.user_id IS '用户ID';


--
-- Name: COLUMN cli_user.created_at; Type: COMMENT;;
--

COMMENT ON COLUMN cli_user.created_at IS '创建时间';


--
-- Name: COLUMN cli_user.updated_at; Type: COMMENT;;
--

COMMENT ON COLUMN cli_user.updated_at IS '更新时间';


--
-- Name: cli_user_oauth; Type: TABLE;;
--

CREATE TABLE cli_user_oauth (
    id character varying(50) NOT NULL,
    cli_user_id character varying(50) NOT NULL,
    cli_oauth_id character varying(50) NOT NULL
);

COMMENT ON TABLE cli_user_oauth IS 'CLI 用户凭证关联表';


--
-- Name: COLUMN cli_user_oauth.id; Type: COMMENT;;
--

COMMENT ON COLUMN cli_user_oauth.id IS 'ID';


--
-- Name: COLUMN cli_user_oauth.cli_user_id; Type: COMMENT;;
--

COMMENT ON COLUMN cli_user_oauth.cli_user_id IS 'CLI 用户ID';


--
-- Name: COLUMN cli_user_oauth.cli_oauth_id; Type: COMMENT;;
--

COMMENT ON COLUMN cli_user_oauth.cli_oauth_id IS 'CLI 认证ID';



--
-- Name: currency_stripe_config; Type: TABLE;;
--

CREATE TABLE currency_stripe_config (
    currency character varying(3) NOT NULL,
    stripe_price_id character varying(255) DEFAULT ''::character varying NOT NULL,
    unit_price numeric(18,6) DEFAULT 0 NOT NULL,
    symbol character varying(10) NOT NULL,
    updated_at timestamp with time zone DEFAULT now()
);

INSERT INTO "currency_stripe_config" ("currency", "stripe_price_id", "unit_price", "symbol", "updated_at") VALUES ('USD', '', '1.000000', '$', '2026-04-27 13:15:35.539079+08');
INSERT INTO "currency_stripe_config" ("currency", "stripe_price_id", "unit_price", "symbol", "updated_at") VALUES ('CNY', '', '7.300000', '¥', '2026-05-09 13:55:56.754377+08');



--
-- Name: consume_rebates; Type: TABLE;;
--

CREATE TABLE consume_rebates (
    id integer NOT NULL,
    inviter_id bigint DEFAULT 0,
    invitee_id bigint DEFAULT 0,
    request_id character varying(64) DEFAULT ''::character varying,
    level bigint DEFAULT 1,
    source_quota bigint DEFAULT 0,
    rebate_ratio numeric DEFAULT 0,
    rebate_quota bigint DEFAULT 0,
    created_at bigint DEFAULT 0,
    provider_id bigint DEFAULT 0,
    provider_pricing_id bigint DEFAULT 0,
    public_model_name character varying(255) DEFAULT ''::character varying,
    base_model_name character varying(255) DEFAULT ''::character varying
);


--
-- Name: TABLE consume_rebates; Type: COMMENT;;
--

COMMENT ON TABLE consume_rebates IS '消费返利记录表：被邀请人使用充值额度消费后，给上级邀请人生成的返利记录';


--
-- Name: COLUMN consume_rebates.id; Type: COMMENT;;
--

COMMENT ON COLUMN consume_rebates.id IS '主键ID';


--
-- Name: COLUMN consume_rebates.inviter_id; Type: COMMENT;;
--

COMMENT ON COLUMN consume_rebates.inviter_id IS '获得返利的邀请人用户ID';


--
-- Name: COLUMN consume_rebates.invitee_id; Type: COMMENT;;
--

COMMENT ON COLUMN consume_rebates.invitee_id IS '产生消费的被邀请人用户ID';


--
-- Name: COLUMN consume_rebates.request_id; Type: COMMENT;;
--

COMMENT ON COLUMN consume_rebates.request_id IS '消费请求ID：用于同一次消费返利幂等去重';


--
-- Name: COLUMN consume_rebates.level; Type: COMMENT;;
--

COMMENT ON COLUMN consume_rebates.level IS '返利层级：1表示一级消费返利，2表示二级消费返利';


--
-- Name: COLUMN consume_rebates.source_quota; Type: COMMENT;;
--

COMMENT ON COLUMN consume_rebates.source_quota IS '参与返利计算的原始消费额度，只统计充值额度消费，不统计订阅额度和奖励额度';


--
-- Name: COLUMN consume_rebates.rebate_ratio; Type: COMMENT;;
--

COMMENT ON COLUMN consume_rebates.rebate_ratio IS '返利比例，单位为百分比';


--
-- Name: COLUMN consume_rebates.rebate_quota; Type: COMMENT;;
--

COMMENT ON COLUMN consume_rebates.rebate_quota IS '本次实际返利额度';


--
-- Name: COLUMN consume_rebates.created_at; Type: COMMENT;;
--

COMMENT ON COLUMN consume_rebates.created_at IS '创建时间，Unix时间戳秒';


--
-- Name: COLUMN consume_rebates.provider_id; Type: COMMENT;;
--

COMMENT ON COLUMN consume_rebates.provider_id IS '所属服务商 ID，0 表示主站消费返利记录';


--
-- Name: consume_rebates_id_seq; Type: SEQUENCE;;
--

ALTER TABLE consume_rebates ALTER COLUMN id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME consume_rebates_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: crypto_chain_config; Type: TABLE;;
--

CREATE TABLE crypto_chain_config (
    network character varying(32) NOT NULL,
    chain_id bigint NOT NULL,
    token_symbol character varying(20) DEFAULT 'USDT'::character varying NOT NULL,
    token_decimals smallint DEFAULT 18 NOT NULL,
    token_contract character varying(128) DEFAULT ''::character varying NOT NULL,
    receiver_address character varying(128) DEFAULT ''::character varying NOT NULL,
    rpc_url character varying(512) DEFAULT ''::character varying NOT NULL,
    min_confirmations smallint DEFAULT 3 NOT NULL
);


COMMENT ON TABLE crypto_chain_config IS '加密货币链配置表：每条链每种代币一行，network + token_symbol 唯一确定一组参数';


--
-- Name: COLUMN crypto_chain_config.network; Type: COMMENT;;
--

COMMENT ON COLUMN crypto_chain_config.network IS '网络名称（复合主键），如 Sepolia / BSC / Polygon，大小写不敏感匹配';


--
-- Name: COLUMN crypto_chain_config.chain_id; Type: COMMENT;;
--

COMMENT ON COLUMN crypto_chain_config.chain_id IS 'EIP-155 链 ID（Sepolia=11155111, BSC=97, Polygon=137）';


--
-- Name: COLUMN crypto_chain_config.token_symbol; Type: COMMENT;;
--

COMMENT ON COLUMN crypto_chain_config.token_symbol IS '代币符号（复合主键），如 USDT / USDC';


--
-- Name: COLUMN crypto_chain_config.token_decimals; Type: COMMENT;;
--

COMMENT ON COLUMN crypto_chain_config.token_decimals IS '代币精度（Sepolia MockUSDT=6, BSC USDT=18, Polygon USDT=6）';


--
-- Name: COLUMN crypto_chain_config.token_contract; Type: COMMENT;;
--

COMMENT ON COLUMN crypto_chain_config.token_contract IS '代币合约地址';


--
-- Name: COLUMN crypto_chain_config.receiver_address; Type: COMMENT;;
--

COMMENT ON COLUMN crypto_chain_config.receiver_address IS '收款钱包地址';


--
-- Name: COLUMN crypto_chain_config.rpc_url; Type: COMMENT;;
--

COMMENT ON COLUMN crypto_chain_config.rpc_url IS '链节点 RPC 地址（含 API Key）';


--
-- Name: COLUMN crypto_chain_config.min_confirmations; Type: COMMENT;;
--

COMMENT ON COLUMN crypto_chain_config.min_confirmations IS '最小链上确认数（测试网建议 1~2，主网建议 ≥3）';


--
-- Name: crypto_transactions_id_seq; Type: SEQUENCE;;
--

CREATE SEQUENCE crypto_transactions_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


CREATE TABLE crypto_transactions (
    id bigint DEFAULT nextval('crypto_transactions_id_seq'::regclass) NOT NULL,
    top_up_id bigint NOT NULL,
    subscription_order_id bigint DEFAULT 0,
    user_id bigint NOT NULL,
    trade_no character varying(255) NOT NULL,
    tx_hash character varying(128),
    chain_id bigint NOT NULL,
    token_symbol character varying(20) NOT NULL,
    token_contract character varying(128) NOT NULL,
    receiver_address character varying(128) NOT NULL,
    payer_address character varying(128),
    usdt_amount character varying(64) NOT NULL,
    block_number bigint DEFAULT 0,
    confirmations bigint DEFAULT 0,
    status character varying(20) NOT NULL,
    create_time bigint,
    complete_time bigint,
    updated_at timestamp(6) with time zone
);


COMMENT ON TABLE crypto_transactions IS '加密货币链上交易记录';


--
-- Name: COLUMN crypto_transactions.id; Type: COMMENT;;
--

COMMENT ON COLUMN crypto_transactions.id IS '主键';


--
-- Name: COLUMN crypto_transactions.top_up_id; Type: COMMENT;;
--

COMMENT ON COLUMN crypto_transactions.top_up_id IS '关联的充值订单 ID';


--
-- Name: COLUMN crypto_transactions.subscription_order_id; Type: COMMENT;;
--

COMMENT ON COLUMN crypto_transactions.subscription_order_id IS '关联的订阅订单 ID';


--
-- Name: COLUMN crypto_transactions.user_id; Type: COMMENT;;
--

COMMENT ON COLUMN crypto_transactions.user_id IS '用户 ID';


--
-- Name: COLUMN crypto_transactions.trade_no; Type: COMMENT;;
--

COMMENT ON COLUMN crypto_transactions.trade_no IS '订单号（唯一）';


--
-- Name: COLUMN crypto_transactions.tx_hash; Type: COMMENT;;
--

COMMENT ON COLUMN crypto_transactions.tx_hash IS '链上交易哈希（确认后填入，唯一）';


--
-- Name: COLUMN crypto_transactions.chain_id; Type: COMMENT;;
--

COMMENT ON COLUMN crypto_transactions.chain_id IS '链 ID（如 BSC 主网为 56）';


--
-- Name: COLUMN crypto_transactions.token_symbol; Type: COMMENT;;
--

COMMENT ON COLUMN crypto_transactions.token_symbol IS '代币符号（如 USDT）';


--
-- Name: COLUMN crypto_transactions.token_contract; Type: COMMENT;;
--

COMMENT ON COLUMN crypto_transactions.token_contract IS '代币合约地址';


--
-- Name: COLUMN crypto_transactions.receiver_address; Type: COMMENT;;
--

COMMENT ON COLUMN crypto_transactions.receiver_address IS '收款地址';


--
-- Name: COLUMN crypto_transactions.payer_address; Type: COMMENT;;
--

COMMENT ON COLUMN crypto_transactions.payer_address IS '付款地址（链上确认后填入）';


--
-- Name: COLUMN crypto_transactions.usdt_amount; Type: COMMENT;;
--

COMMENT ON COLUMN crypto_transactions.usdt_amount IS 'USDT 金额（字符串存储，保证精度）';


--
-- Name: COLUMN crypto_transactions.block_number; Type: COMMENT;;
--

COMMENT ON COLUMN crypto_transactions.block_number IS '区块号（确认后填入）';


--
-- Name: COLUMN crypto_transactions.confirmations; Type: COMMENT;;
--

COMMENT ON COLUMN crypto_transactions.confirmations IS '确认数（确认后填入）';


--
-- Name: COLUMN crypto_transactions.status; Type: COMMENT;;
--

COMMENT ON COLUMN crypto_transactions.status IS '状态：pending-待确认 / success-已完成 / failed-失败';


--
-- Name: COLUMN crypto_transactions.create_time; Type: COMMENT;;
--

COMMENT ON COLUMN crypto_transactions.create_time IS '创建时间（Unix 时间戳）';


--
-- Name: COLUMN crypto_transactions.complete_time; Type: COMMENT;;
--

COMMENT ON COLUMN crypto_transactions.complete_time IS '完成时间（Unix 时间戳）';


--
-- Name: COLUMN crypto_transactions.updated_at; Type: COMMENT;;
--

COMMENT ON COLUMN crypto_transactions.updated_at IS '记录更新时间';




--
-- Name: custom_oauth_providers; Type: TABLE;;
--

CREATE TABLE custom_oauth_providers (
    id bigint NOT NULL,
    name character varying(64) NOT NULL,
    slug character varying(64) NOT NULL,
    icon character varying(128) DEFAULT ''::character varying,
    enabled boolean DEFAULT false,
    client_id character varying(256),
    client_secret character varying(512),
    authorization_endpoint character varying(512),
    token_endpoint character varying(512),
    user_info_endpoint character varying(512),
    scopes character varying(256) DEFAULT 'openid profile email'::character varying,
    user_id_field character varying(128) DEFAULT 'sub'::character varying,
    username_field character varying(128) DEFAULT 'preferred_username'::character varying,
    display_name_field character varying(128) DEFAULT 'name'::character varying,
    email_field character varying(128) DEFAULT 'email'::character varying,
    well_known character varying(512),
    auth_style bigint DEFAULT 0,
    access_policy text,
    access_denied_message character varying(512),
    created_at timestamp with time zone,
    updated_at timestamp with time zone
);


--
-- Name: custom_oauth_providers_id_seq; Type: SEQUENCE;;
--

CREATE SEQUENCE custom_oauth_providers_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: custom_oauth_providers_id_seq; Type: SEQUENCE OWNED BY;;
--

ALTER SEQUENCE custom_oauth_providers_id_seq OWNED BY custom_oauth_providers.id;


--
-- Name: epay_merchants; Type: TABLE;;
--

CREATE TABLE epay_merchants (
    id bigint NOT NULL,
    pid character varying(32) NOT NULL,
    name character varying(128) DEFAULT ''::character varying NOT NULL,
    key character varying(64) NOT NULL,
    active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone
);


CREATE SEQUENCE epay_merchants_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;



ALTER SEQUENCE epay_merchants_id_seq OWNED BY epay_merchants.id;


CREATE TABLE invite_records (
    id integer NOT NULL,
    inviter_id bigint,
    invitee_id bigint,
    register_time bigint NOT NULL,
    reward_quota bigint DEFAULT 0,
    created_at bigint NOT NULL,
    provider_id bigint DEFAULT 0 NOT NULL
);

--
-- Name: COLUMN invite_records.provider_id; Type: COMMENT;;
--

COMMENT ON COLUMN invite_records.provider_id IS '所属服务商 ID，0 表示主站邀请记录';


--
-- Name: invite_records_id_seq; Type: SEQUENCE;;
--

ALTER TABLE invite_records ALTER COLUMN id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME invite_records_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


CREATE TABLE login_audit_logs (
    id bigint NOT NULL,
    username character varying(64) NOT NULL,
    success boolean NOT NULL,
    ip character varying(45) DEFAULT ''::character varying NOT NULL,
    user_agent character varying(512) DEFAULT ''::character varying NOT NULL,
    reason character varying(256) DEFAULT ''::character varying NOT NULL,
    created_at timestamp with time zone
);

CREATE SEQUENCE login_audit_logs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE login_audit_logs_id_seq OWNED BY login_audit_logs.id;


CREATE TABLE logs (
    id bigint NOT NULL,
    user_id bigint,
    created_at bigint,
    type bigint,
    content text,
    username text DEFAULT ''::text,
    token_name text DEFAULT ''::text,
    model_name text DEFAULT ''::text,
    quota bigint DEFAULT 0,
    prompt_tokens bigint DEFAULT 0,
    completion_tokens bigint DEFAULT 0,
    use_time bigint DEFAULT 0,
    is_stream boolean,
    channel_id bigint,
    channel_name text,
    token_id bigint DEFAULT 0,
    "group" text,
    ip text DEFAULT ''::text,
    request_id character varying(64) DEFAULT ''::character varying,
    other text,
    provider_id bigint DEFAULT 0,
    base_model_name text DEFAULT ''::text,
    billing_side character varying(32) DEFAULT ''::character varying,
    upstream_request_id character varying(128) DEFAULT ''::character varying,
    provider_name text
);


--
-- Name: COLUMN logs.provider_id; Type: COMMENT;;
--

COMMENT ON COLUMN logs.provider_id IS '日志所属服务商 ID；0 表示主站日志，非 0 表示服务商域名下产生的日志';


--
-- Name: COLUMN logs.base_model_name; Type: COMMENT;;
--

COMMENT ON COLUMN logs.base_model_name IS '服务商场景下的主站真实模型名；普通主站日志可为空';


--
-- Name: COLUMN logs.billing_side; Type: COMMENT;;
--

COMMENT ON COLUMN logs.billing_side IS '服务商账本方向：provider_user 表示服务商用户售价记录，provider_cost 表示服务商主站成本记录';




CREATE SEQUENCE logs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: logs_id_seq; Type: SEQUENCE OWNED BY;;
--

ALTER SEQUENCE logs_id_seq OWNED BY logs.id;



CREATE TABLE midjourneys (
    id bigint NOT NULL,
    code bigint,
    user_id bigint,
    action character varying(40),
    mj_id text,
    prompt text,
    prompt_en text,
    description text,
    state text,
    submit_time bigint,
    start_time bigint,
    finish_time bigint,
    image_url text,
    video_url text,
    video_urls text,
    status character varying(20),
    progress character varying(30),
    fail_reason text,
    channel_id bigint,
    quota bigint,
    buttons text,
    properties text
);

--
-- Name: midjourneys_id_seq; Type: SEQUENCE;;
--

CREATE SEQUENCE midjourneys_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;



ALTER SEQUENCE midjourneys_id_seq OWNED BY midjourneys.id;



CREATE TABLE models (
    id bigint NOT NULL,
    model_name character varying(128) NOT NULL,
    description text,
    icon character varying(128),
    tags character varying(255),
    vendor_id bigint,
    endpoints text,
    status bigint DEFAULT 1,
    sync_official bigint DEFAULT 1,
    created_time bigint,
    updated_time bigint,
    deleted_at timestamp with time zone,
    name_rule bigint DEFAULT 0,
    description_i18n text,
    features_i18n text
);


CREATE SEQUENCE models_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;



--
-- Name: models_id_seq; Type: SEQUENCE OWNED BY;;
--

ALTER SEQUENCE models_id_seq OWNED BY models.id;


CREATE TABLE options (
    key text NOT NULL,
    value text
);


--
-- Name: orders; Type: TABLE;;
--

CREATE TABLE orders (
    id bigint NOT NULL,
    out_trade_no character varying(64) NOT NULL,
    trade_no character varying(128) DEFAULT ''::character varying NOT NULL,
    pid character varying(64) DEFAULT ''::character varying NOT NULL,
    type character varying(32) DEFAULT 'wxpay'::character varying NOT NULL,
    name character varying(256) DEFAULT ''::character varying NOT NULL,
    money character varying(32) DEFAULT ''::character varying NOT NULL,
    notify_url character varying(512) DEFAULT ''::character varying NOT NULL,
    return_url character varying(512) DEFAULT ''::character varying NOT NULL,
    status character varying(32) DEFAULT 'pending'::character varying NOT NULL,
    callback_status character varying(32) DEFAULT 'pending'::character varying NOT NULL,
    callback_resp text DEFAULT ''::text NOT NULL,
    callback_time bigint DEFAULT 0 NOT NULL,
    code_url text DEFAULT ''::text NOT NULL,
    provider_payload text DEFAULT ''::text NOT NULL,
    usdt_amount character varying(32) DEFAULT ''::character varying NOT NULL,
    usdt_address character varying(64) DEFAULT ''::character varying NOT NULL,
    usdt_tx_hash character varying(128) DEFAULT ''::character varying NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone
);


--
-- Name: orders_id_seq; Type: SEQUENCE;;
--

CREATE SEQUENCE orders_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;



ALTER SEQUENCE orders_id_seq OWNED BY orders.id;


--
-- Name: passkey_credentials; Type: TABLE;;
--

CREATE TABLE passkey_credentials (
    id bigint NOT NULL,
    user_id bigint NOT NULL,
    credential_id character varying(512) NOT NULL,
    public_key text NOT NULL,
    attestation_type character varying(255),
    aa_guid character varying(512),
    sign_count bigint DEFAULT 0,
    clone_warning boolean,
    user_present boolean,
    user_verified boolean,
    backup_eligible boolean,
    backup_state boolean,
    transports text,
    attachment character varying(32),
    last_used_at timestamp with time zone,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone
);


CREATE SEQUENCE passkey_credentials_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE passkey_credentials_id_seq OWNED BY passkey_credentials.id;


CREATE TABLE payment_bill_reconcile (
    id bigint NOT NULL,
    channel_type character varying(32) DEFAULT ''::character varying,
    bill_record_id bigint DEFAULT 0,
    bill_date character varying(16) DEFAULT ''::character varying,
    trade_time character varying(64) DEFAULT ''::character varying,
    channel_trade_no character varying(128) DEFAULT ''::character varying,
    merchant_trade_no character varying(128) DEFAULT ''::character varying,
    trade_type character varying(64) DEFAULT ''::character varying,
    channel_status character varying(64) DEFAULT ''::character varying,
    channel_refund_status character varying(64) DEFAULT ''::character varying,
    channel_amount character varying(64) DEFAULT ''::character varying,
    channel_refund_amount character varying(64) DEFAULT ''::character varying,
    local_type character varying(32) DEFAULT ''::character varying,
    local_id bigint DEFAULT 0,
    local_trade_no character varying(255) DEFAULT ''::character varying,
    local_payment_method character varying(50) DEFAULT ''::character varying,
    local_status character varying(64) DEFAULT ''::character varying,
    local_amount numeric(18,6) DEFAULT 0,
    local_create_time bigint DEFAULT 0,
    local_complete_time bigint DEFAULT 0,
    reconcile_status character varying(32) DEFAULT ''::character varying,
    reconcile_reason character varying(64) DEFAULT ''::character varying,
    remark text DEFAULT ''::text,
    created_at bigint DEFAULT 0,
    updated_at bigint DEFAULT 0,
    reconcile_key character varying(128),
    record_source character varying(32),
    channel_currency character varying(16) DEFAULT ''::character varying,
    local_currency character varying(16) DEFAULT ''::character varying
);

--
-- Name: TABLE payment_bill_reconcile; Type: COMMENT;;
--

COMMENT ON TABLE payment_bill_reconcile IS '通用支付渠道对账结果表';


--
-- Name: COLUMN payment_bill_reconcile.id; Type: COMMENT;;
--

COMMENT ON COLUMN payment_bill_reconcile.id IS '主键ID';


--
-- Name: COLUMN payment_bill_reconcile.channel_type; Type: COMMENT;;
--

COMMENT ON COLUMN payment_bill_reconcile.channel_type IS '支付渠道类型，如 wxpay、stripe';


--
-- Name: COLUMN payment_bill_reconcile.bill_record_id; Type: COMMENT;;
--

COMMENT ON COLUMN payment_bill_reconcile.bill_record_id IS '关联 payment_bill_record.id';


--
-- Name: COLUMN payment_bill_reconcile.bill_date; Type: COMMENT;;
--

COMMENT ON COLUMN payment_bill_reconcile.bill_date IS '账单日期，格式 YYYY-MM-DD';


--
-- Name: COLUMN payment_bill_reconcile.trade_time; Type: COMMENT;;
--

COMMENT ON COLUMN payment_bill_reconcile.trade_time IS '渠道账单中的交易时间';


--
-- Name: COLUMN payment_bill_reconcile.channel_trade_no; Type: COMMENT;;
--

COMMENT ON COLUMN payment_bill_reconcile.channel_trade_no IS '支付渠道交易单号';


--
-- Name: COLUMN payment_bill_reconcile.merchant_trade_no; Type: COMMENT;;
--

COMMENT ON COLUMN payment_bill_reconcile.merchant_trade_no IS '商户订单号';


--
-- Name: COLUMN payment_bill_reconcile.trade_type; Type: COMMENT;;
--

COMMENT ON COLUMN payment_bill_reconcile.trade_type IS '交易类型';


--
-- Name: COLUMN payment_bill_reconcile.channel_status; Type: COMMENT;;
--

COMMENT ON COLUMN payment_bill_reconcile.channel_status IS '渠道侧交易状态';


--
-- Name: COLUMN payment_bill_reconcile.channel_refund_status; Type: COMMENT;;
--

COMMENT ON COLUMN payment_bill_reconcile.channel_refund_status IS '渠道侧退款状态';


--
-- Name: COLUMN payment_bill_reconcile.channel_amount; Type: COMMENT;;
--

COMMENT ON COLUMN payment_bill_reconcile.channel_amount IS '渠道侧交易金额';


--
-- Name: COLUMN payment_bill_reconcile.channel_refund_amount; Type: COMMENT;;
--

COMMENT ON COLUMN payment_bill_reconcile.channel_refund_amount IS '渠道侧退款金额';


--
-- Name: COLUMN payment_bill_reconcile.local_type; Type: COMMENT;;
--

COMMENT ON COLUMN payment_bill_reconcile.local_type IS '本地业务类型，如 topup、subscription';


--
-- Name: COLUMN payment_bill_reconcile.local_id; Type: COMMENT;;
--

COMMENT ON COLUMN payment_bill_reconcile.local_id IS '本地业务表主键ID';


--
-- Name: COLUMN payment_bill_reconcile.local_trade_no; Type: COMMENT;;
--

COMMENT ON COLUMN payment_bill_reconcile.local_trade_no IS '本地订单号';


--
-- Name: COLUMN payment_bill_reconcile.local_payment_method; Type: COMMENT;;
--

COMMENT ON COLUMN payment_bill_reconcile.local_payment_method IS '本地支付方式';


--
-- Name: COLUMN payment_bill_reconcile.local_status; Type: COMMENT;;
--

COMMENT ON COLUMN payment_bill_reconcile.local_status IS '本地订单状态';


--
-- Name: COLUMN payment_bill_reconcile.local_amount; Type: COMMENT;;
--

COMMENT ON COLUMN payment_bill_reconcile.local_amount IS '本地订单金额';


--
-- Name: COLUMN payment_bill_reconcile.local_create_time; Type: COMMENT;;
--

COMMENT ON COLUMN payment_bill_reconcile.local_create_time IS '本地订单创建时间戳';


--
-- Name: COLUMN payment_bill_reconcile.local_complete_time; Type: COMMENT;;
--

COMMENT ON COLUMN payment_bill_reconcile.local_complete_time IS '本地订单完成时间戳';


--
-- Name: COLUMN payment_bill_reconcile.reconcile_status; Type: COMMENT;;
--

COMMENT ON COLUMN payment_bill_reconcile.reconcile_status IS '对账结果状态，如 matched、abnormal';


--
-- Name: COLUMN payment_bill_reconcile.reconcile_reason; Type: COMMENT;;
--

COMMENT ON COLUMN payment_bill_reconcile.reconcile_reason IS '对账结果原因，如 amount_mismatch、local_not_found';


--
-- Name: COLUMN payment_bill_reconcile.remark; Type: COMMENT;;
--

COMMENT ON COLUMN payment_bill_reconcile.remark IS '对账备注说明';


--
-- Name: COLUMN payment_bill_reconcile.created_at; Type: COMMENT;;
--

COMMENT ON COLUMN payment_bill_reconcile.created_at IS '创建时间戳';


--
-- Name: COLUMN payment_bill_reconcile.updated_at; Type: COMMENT;;
--

COMMENT ON COLUMN payment_bill_reconcile.updated_at IS '更新时间戳';


--
-- Name: COLUMN payment_bill_reconcile.reconcile_key; Type: COMMENT;;
--

COMMENT ON COLUMN payment_bill_reconcile.reconcile_key IS '对账记录唯一键，本地单和渠道单边记录都靠它幂等更新';


--
-- Name: COLUMN payment_bill_reconcile.record_source; Type: COMMENT;;
--

COMMENT ON COLUMN payment_bill_reconcile.record_source IS '记录来源，local=本地订单基准记录，channel=微信账单单边记录';


--
-- Name: COLUMN payment_bill_reconcile.channel_currency; Type: COMMENT;;
--

COMMENT ON COLUMN payment_bill_reconcile.channel_currency IS '渠道侧交易币种';


--
-- Name: COLUMN payment_bill_reconcile.local_currency; Type: COMMENT;;
--

COMMENT ON COLUMN payment_bill_reconcile.local_currency IS '本地交易币种';


--
-- Name: payment_bill_reconcile_id_seq; Type: SEQUENCE;;
--

ALTER TABLE payment_bill_reconcile ALTER COLUMN id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME payment_bill_reconcile_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: payment_bill_record; Type: TABLE;;
--

CREATE TABLE payment_bill_record (
    id bigint NOT NULL,
    channel_type character varying(32) DEFAULT ''::character varying,
    bill_date character varying(16) DEFAULT ''::character varying,
    file_path character varying(255) DEFAULT ''::character varying,
    row_index bigint DEFAULT 0,
    row_hash character varying(64) DEFAULT ''::character varying,
    trade_time character varying(64) DEFAULT ''::character varying,
    app_id character varying(64) DEFAULT ''::character varying,
    mch_id character varying(64) DEFAULT ''::character varying,
    sub_mch_id character varying(64) DEFAULT ''::character varying,
    device_id character varying(64) DEFAULT ''::character varying,
    channel_trade_no character varying(128) DEFAULT ''::character varying,
    merchant_trade_no character varying(255) DEFAULT ''::character varying,
    payer_id character varying(128) DEFAULT ''::character varying,
    trade_type character varying(64) DEFAULT ''::character varying,
    trade_status character varying(64) DEFAULT ''::character varying,
    refund_status character varying(64) DEFAULT ''::character varying,
    refund_type character varying(64) DEFAULT ''::character varying,
    currency character varying(16) DEFAULT ''::character varying,
    bank character varying(128) DEFAULT ''::character varying,
    total_amount character varying(64) DEFAULT ''::character varying,
    order_amount character varying(64) DEFAULT ''::character varying,
    refund_amount character varying(64) DEFAULT ''::character varying,
    service_fee character varying(64) DEFAULT ''::character varying,
    rate character varying(64) DEFAULT ''::character varying,
    rate_remark text DEFAULT ''::text,
    goods_name text DEFAULT ''::text,
    package_data text DEFAULT ''::text,
    channel_refund_no character varying(64) DEFAULT ''::character varying,
    merchant_refund_no character varying(64) DEFAULT ''::character varying,
    enterprise_red_packet character varying(64) DEFAULT ''::character varying,
    enterprise_refund character varying(64) DEFAULT ''::character varying,
    apply_refund_amount character varying(64) DEFAULT ''::character varying,
    raw_line text DEFAULT ''::text,
    raw_data_json text DEFAULT ''::text,
    created_at bigint DEFAULT 0
);



--
-- Name: TABLE payment_bill_record; Type: COMMENT;;
--

COMMENT ON TABLE payment_bill_record IS '通用支付渠道账单明细表';


--
-- Name: COLUMN payment_bill_record.id; Type: COMMENT;;
--

COMMENT ON COLUMN payment_bill_record.id IS '主键ID';


--
-- Name: COLUMN payment_bill_record.channel_type; Type: COMMENT;;
--

COMMENT ON COLUMN payment_bill_record.channel_type IS '支付渠道类型，如 wxpay、stripe';


--
-- Name: COLUMN payment_bill_record.bill_date; Type: COMMENT;;
--

COMMENT ON COLUMN payment_bill_record.bill_date IS '账单日期，格式 YYYY-MM-DD';


--
-- Name: COLUMN payment_bill_record.file_path; Type: COMMENT;;
--

COMMENT ON COLUMN payment_bill_record.file_path IS '本地保存的账单文件路径';


--
-- Name: COLUMN payment_bill_record.row_index; Type: COMMENT;;
--

COMMENT ON COLUMN payment_bill_record.row_index IS '账单文件中的行号';


--
-- Name: COLUMN payment_bill_record.row_hash; Type: COMMENT;;
--

COMMENT ON COLUMN payment_bill_record.row_hash IS '账单行内容哈希，用于去重';


--
-- Name: COLUMN payment_bill_record.trade_time; Type: COMMENT;;
--

COMMENT ON COLUMN payment_bill_record.trade_time IS '渠道账单中的交易时间';


--
-- Name: COLUMN payment_bill_record.app_id; Type: COMMENT;;
--

COMMENT ON COLUMN payment_bill_record.app_id IS '应用ID，例如微信公众账号ID';


--
-- Name: COLUMN payment_bill_record.mch_id; Type: COMMENT;;
--

COMMENT ON COLUMN payment_bill_record.mch_id IS '商户号';


--
-- Name: COLUMN payment_bill_record.sub_mch_id; Type: COMMENT;;
--

COMMENT ON COLUMN payment_bill_record.sub_mch_id IS '子商户号/特约商户号';


--
-- Name: COLUMN payment_bill_record.device_id; Type: COMMENT;;
--

COMMENT ON COLUMN payment_bill_record.device_id IS '设备号';


--
-- Name: COLUMN payment_bill_record.channel_trade_no; Type: COMMENT;;
--

COMMENT ON COLUMN payment_bill_record.channel_trade_no IS '支付渠道交易单号，例如微信订单号';


--
-- Name: COLUMN payment_bill_record.merchant_trade_no; Type: COMMENT;;
--

COMMENT ON COLUMN payment_bill_record.merchant_trade_no IS '商户订单号';


--
-- Name: COLUMN payment_bill_record.payer_id; Type: COMMENT;;
--

COMMENT ON COLUMN payment_bill_record.payer_id IS '支付用户标识，例如微信 openid';


--
-- Name: COLUMN payment_bill_record.trade_type; Type: COMMENT;;
--

COMMENT ON COLUMN payment_bill_record.trade_type IS '交易类型';


--
-- Name: COLUMN payment_bill_record.trade_status; Type: COMMENT;;
--

COMMENT ON COLUMN payment_bill_record.trade_status IS '交易状态';


--
-- Name: COLUMN payment_bill_record.refund_status; Type: COMMENT;;
--

COMMENT ON COLUMN payment_bill_record.refund_status IS '退款状态';


--
-- Name: COLUMN payment_bill_record.refund_type; Type: COMMENT;;
--

COMMENT ON COLUMN payment_bill_record.refund_type IS '退款类型';


--
-- Name: COLUMN payment_bill_record.currency; Type: COMMENT;;
--

COMMENT ON COLUMN payment_bill_record.currency IS '货币类型';


--
-- Name: COLUMN payment_bill_record.bank; Type: COMMENT;;
--

COMMENT ON COLUMN payment_bill_record.bank IS '付款银行';


--
-- Name: COLUMN payment_bill_record.total_amount; Type: COMMENT;;
--

COMMENT ON COLUMN payment_bill_record.total_amount IS '总金额';


--
-- Name: COLUMN payment_bill_record.order_amount; Type: COMMENT;;
--

COMMENT ON COLUMN payment_bill_record.order_amount IS '订单金额';


--
-- Name: COLUMN payment_bill_record.refund_amount; Type: COMMENT;;
--

COMMENT ON COLUMN payment_bill_record.refund_amount IS '退款金额';


--
-- Name: COLUMN payment_bill_record.service_fee; Type: COMMENT;;
--

COMMENT ON COLUMN payment_bill_record.service_fee IS '手续费';


--
-- Name: COLUMN payment_bill_record.rate; Type: COMMENT;;
--

COMMENT ON COLUMN payment_bill_record.rate IS '费率';


--
-- Name: COLUMN payment_bill_record.rate_remark; Type: COMMENT;;
--

COMMENT ON COLUMN payment_bill_record.rate_remark IS '费率备注';


--
-- Name: COLUMN payment_bill_record.goods_name; Type: COMMENT;;
--

COMMENT ON COLUMN payment_bill_record.goods_name IS '商品名称';


--
-- Name: COLUMN payment_bill_record.package_data; Type: COMMENT;;
--

COMMENT ON COLUMN payment_bill_record.package_data IS '商户数据包/附加信息';


--
-- Name: COLUMN payment_bill_record.channel_refund_no; Type: COMMENT;;
--

COMMENT ON COLUMN payment_bill_record.channel_refund_no IS '支付渠道退款单号，例如微信退款单号';


--
-- Name: COLUMN payment_bill_record.merchant_refund_no; Type: COMMENT;;
--

COMMENT ON COLUMN payment_bill_record.merchant_refund_no IS '商户退款单号';


--
-- Name: COLUMN payment_bill_record.enterprise_red_packet; Type: COMMENT;;
--

COMMENT ON COLUMN payment_bill_record.enterprise_red_packet IS '企业红包金额';


--
-- Name: COLUMN payment_bill_record.enterprise_refund; Type: COMMENT;;
--

COMMENT ON COLUMN payment_bill_record.enterprise_refund IS '企业红包退款金额';


--
-- Name: COLUMN payment_bill_record.apply_refund_amount; Type: COMMENT;;
--

COMMENT ON COLUMN payment_bill_record.apply_refund_amount IS '申请退款金额';


--
-- Name: COLUMN payment_bill_record.raw_line; Type: COMMENT;;
--

COMMENT ON COLUMN payment_bill_record.raw_line IS '账单原始整行文本';


--
-- Name: COLUMN payment_bill_record.raw_data_json; Type: COMMENT;;
--

COMMENT ON COLUMN payment_bill_record.raw_data_json IS '账单原始字段 JSON';


--
-- Name: COLUMN payment_bill_record.created_at; Type: COMMENT;;
--

COMMENT ON COLUMN payment_bill_record.created_at IS '创建时间戳';


--
-- Name: payment_bill_record_id_seq; Type: SEQUENCE;;
--

ALTER TABLE payment_bill_record ALTER COLUMN id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME payment_bill_record_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: prefill_groups; Type: TABLE;;
--

CREATE TABLE prefill_groups (
    id bigint NOT NULL,
    name character varying(64) NOT NULL,
    type character varying(32) NOT NULL,
    items json,
    description character varying(255),
    created_time bigint,
    updated_time bigint,
    deleted_at timestamp with time zone
);


--
-- Name: prefill_groups_id_seq; Type: SEQUENCE;;
--

CREATE SEQUENCE prefill_groups_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: prefill_groups_id_seq; Type: SEQUENCE OWNED BY;;
--

ALTER SEQUENCE prefill_groups_id_seq OWNED BY prefill_groups.id;


--
-- Name: provider_configs; Type: TABLE;;
--

CREATE TABLE provider_configs (
    id bigint NOT NULL,
    provider_id bigint NOT NULL,
    site_name character varying(128),
    logo text,
    theme_color character varying(32),
    login_background text,
    home_modules text,
    nav_modules text,
    pricing_display text,
    announcement text,
    footer_text text,
    support_url text,
    created_at bigint,
    updated_at bigint,
    secondary_color character varying(32) DEFAULT ''::character varying,
    wechat_support text DEFAULT ''::character varying,
    qq_support text DEFAULT ''::character varying,
    wechat_support_desc text DEFAULT ''::character varying,
    qq_support_qrcode text DEFAULT ''::character varying,
    telegram_support text DEFAULT ''::character varying,
    telegram_support_desc text DEFAULT ''::character varying,
    import_price_ratio numeric(10,6) DEFAULT 1 NOT NULL,
    home_page_theme character varying(64) DEFAULT ''::character varying,
    model_pricing_sync_enabled boolean DEFAULT false,
    model_pricing_sync_last_at bigint DEFAULT 0,
    model_pricing_sync_last_summary text,
    CONSTRAINT chk_provider_configs_import_price_ratio CHECK (((import_price_ratio > (0)::numeric) AND (import_price_ratio <= (1)::numeric)))
);

--
-- Name: TABLE provider_configs; Type: COMMENT;;
--

COMMENT ON TABLE provider_configs IS '服务商页面配置表：控制服务商域名下的站点展示';


--
-- Name: COLUMN provider_configs.id; Type: COMMENT;;
--

COMMENT ON COLUMN provider_configs.id IS '配置 ID，主键';


--
-- Name: COLUMN provider_configs.provider_id; Type: COMMENT;;
--

COMMENT ON COLUMN provider_configs.provider_id IS '所属服务商 ID，唯一关联 providers.id';


--
-- Name: COLUMN provider_configs.site_name; Type: COMMENT;;
--

COMMENT ON COLUMN provider_configs.site_name IS '服务商站点名称，会覆盖前端显示的系统名';


--
-- Name: COLUMN provider_configs.logo; Type: COMMENT;;
--

COMMENT ON COLUMN provider_configs.logo IS '服务商 Logo 地址';


--
-- Name: COLUMN provider_configs.theme_color; Type: COMMENT;;
--

COMMENT ON COLUMN provider_configs.theme_color IS '服务商站点主色，为空时访问服务商站点使用默认主色 #09FEF7';


--
-- Name: COLUMN provider_configs.login_background; Type: COMMENT;;
--

COMMENT ON COLUMN provider_configs.login_background IS '登录页背景图地址';


--
-- Name: COLUMN provider_configs.home_modules; Type: COMMENT;;
--

COMMENT ON COLUMN provider_configs.home_modules IS '首页模块开关配置，JSON 字符串';


--
-- Name: COLUMN provider_configs.nav_modules; Type: COMMENT;;
--

COMMENT ON COLUMN provider_configs.nav_modules IS '导航菜单开关配置，JSON 字符串';


--
-- Name: COLUMN provider_configs.pricing_display; Type: COMMENT;;
--

COMMENT ON COLUMN provider_configs.pricing_display IS '模型价格页展示配置，JSON 字符串';


--
-- Name: COLUMN provider_configs.announcement; Type: COMMENT;;
--

COMMENT ON COLUMN provider_configs.announcement IS '服务商自定义公告文本';


--
-- Name: COLUMN provider_configs.footer_text; Type: COMMENT;;
--

COMMENT ON COLUMN provider_configs.footer_text IS '页脚文案或 HTML 文本';


--
-- Name: COLUMN provider_configs.support_url; Type: COMMENT;;
--

COMMENT ON COLUMN provider_configs.support_url IS '客服链接';


--
-- Name: COLUMN provider_configs.created_at; Type: COMMENT;;
--

COMMENT ON COLUMN provider_configs.created_at IS '创建时间，Unix 秒级时间戳';


--
-- Name: COLUMN provider_configs.updated_at; Type: COMMENT;;
--

COMMENT ON COLUMN provider_configs.updated_at IS '更新时间，Unix 秒级时间戳';


--
-- Name: COLUMN provider_configs.secondary_color; Type: COMMENT;;
--

COMMENT ON COLUMN provider_configs.secondary_color IS '服务商站点辅色，为空时访问服务商站点使用默认辅色 #BAFF29';


--
-- Name: COLUMN provider_configs.wechat_support; Type: COMMENT;;
--

COMMENT ON COLUMN provider_configs.wechat_support IS '微信客服';


--
-- Name: COLUMN provider_configs.qq_support; Type: COMMENT;;
--

COMMENT ON COLUMN provider_configs.qq_support IS 'QQ客服';


--
-- Name: COLUMN provider_configs.wechat_support_desc; Type: COMMENT;;
--

COMMENT ON COLUMN provider_configs.wechat_support_desc IS '微信客服文本描述';


--
-- Name: COLUMN provider_configs.qq_support_qrcode; Type: COMMENT;;
--

COMMENT ON COLUMN provider_configs.qq_support_qrcode IS 'QQ客服二维码';


--
-- Name: COLUMN provider_configs.telegram_support; Type: COMMENT;;
--

COMMENT ON COLUMN provider_configs.telegram_support IS 'Telegram客服';


--
-- Name: COLUMN provider_configs.telegram_support_desc; Type: COMMENT;;
--

COMMENT ON COLUMN provider_configs.telegram_support_desc IS 'Telegram客服文本描述';


--
-- Name: COLUMN provider_configs.import_price_ratio; Type: COMMENT;;
--

COMMENT ON COLUMN provider_configs.import_price_ratio IS '进货价比例';


--
-- Name: COLUMN provider_configs.home_page_theme; Type: COMMENT;;
--

COMMENT ON COLUMN provider_configs.home_page_theme IS '服务商首页选择键，例如 default、b、c，用于对应不同首页内容';


--
-- Name: COLUMN provider_configs.model_pricing_sync_enabled; Type: COMMENT;;
--

COMMENT ON COLUMN provider_configs.model_pricing_sync_enabled IS '模型定价自动同步开关：开启后主站模型新增/下架/恢复时会自动同步该服务商';


--
-- Name: COLUMN provider_configs.model_pricing_sync_last_at; Type: COMMENT;;
--

COMMENT ON COLUMN provider_configs.model_pricing_sync_last_at IS '上次自动同步时间，Unix 秒级时间戳';


--
-- Name: COLUMN provider_configs.model_pricing_sync_last_summary; Type: COMMENT;;
--

COMMENT ON COLUMN provider_configs.model_pricing_sync_last_summary IS '上次自动同步结果摘要，JSON 字符串，包含新增/软禁用/恢复/跳过的模型名及计数';


--
-- Name: provider_configs_id_seq; Type: SEQUENCE;;
--

CREATE SEQUENCE provider_configs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: provider_configs_id_seq; Type: SEQUENCE OWNED BY;;
--

ALTER SEQUENCE provider_configs_id_seq OWNED BY provider_configs.id;


--
-- Name: provider_domains; Type: TABLE;;
--

CREATE TABLE provider_domains (
    id bigint NOT NULL,
    provider_id bigint NOT NULL,
    domain character varying(255) NOT NULL,
    status bigint DEFAULT 0,
    verify_token character varying(64),
    created_at bigint,
    updated_at bigint
);

--
-- Name: TABLE provider_domains; Type: COMMENT;;
--

COMMENT ON TABLE provider_domains IS '服务商域名绑定表：根据请求 Host 解析到对应服务商';


--
-- Name: COLUMN provider_domains.id; Type: COMMENT;;
--

COMMENT ON COLUMN provider_domains.id IS '域名绑定 ID，主键';


--
-- Name: COLUMN provider_domains.provider_id; Type: COMMENT;;
--

COMMENT ON COLUMN provider_domains.provider_id IS '所属服务商 ID，关联 providers.id';


--
-- Name: COLUMN provider_domains.domain; Type: COMMENT;;
--

COMMENT ON COLUMN provider_domains.domain IS '服务商绑定域名，例如 api.example.com；必须全局唯一';


--
-- Name: COLUMN provider_domains.status; Type: COMMENT;;
--

COMMENT ON COLUMN provider_domains.status IS '域名状态：1 已验证可用，0 待验证或禁用';


--
-- Name: COLUMN provider_domains.verify_token; Type: COMMENT;;
--

COMMENT ON COLUMN provider_domains.verify_token IS '域名验证令牌，可用于 TXT 或 CNAME 校验';


--
-- Name: COLUMN provider_domains.created_at; Type: COMMENT;;
--

COMMENT ON COLUMN provider_domains.created_at IS '创建时间，Unix 秒级时间戳';


--
-- Name: COLUMN provider_domains.updated_at; Type: COMMENT;;
--

COMMENT ON COLUMN provider_domains.updated_at IS '更新时间，Unix 秒级时间戳';


--
-- Name: provider_domains_id_seq; Type: SEQUENCE;;
--

CREATE SEQUENCE provider_domains_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: provider_domains_id_seq; Type: SEQUENCE OWNED BY;;
--

ALTER SEQUENCE provider_domains_id_seq OWNED BY provider_domains.id;


--
-- Name: provider_model_pricings; Type: TABLE;;
--

CREATE TABLE provider_model_pricings (
    id bigint NOT NULL,
    provider_id bigint NOT NULL,
    public_model_name character varying(255) NOT NULL,
    base_model_name character varying(255) NOT NULL,
    enabled boolean DEFAULT true,
    pricing_type character varying(16) DEFAULT 'ratio'::character varying NOT NULL,
    ratio numeric(18,8) DEFAULT 1,
    delta_model_ratio numeric(18,8) DEFAULT 0,
    delta_model_price numeric(18,8) DEFAULT 0,
    created_at bigint,
    updated_at bigint,
    consume_rebate_ratio_level1 numeric(10,6) DEFAULT 0 NOT NULL,
    consume_rebate_ratio_level2 numeric(10,6) DEFAULT 0 NOT NULL,
    sync_disabled boolean DEFAULT false,
    CONSTRAINT chk_provider_model_pricings_rebate_l1_range CHECK (((consume_rebate_ratio_level1 >= (0)::numeric) AND (consume_rebate_ratio_level1 <= (100)::numeric))),
    CONSTRAINT chk_provider_model_pricings_rebate_l2_range CHECK (((consume_rebate_ratio_level2 >= (0)::numeric) AND (consume_rebate_ratio_level2 <= (100)::numeric)))
);


--
-- Name: TABLE provider_model_pricings; Type: COMMENT;;
--

COMMENT ON TABLE provider_model_pricings IS '服务商模型定价表：把服务商展示模型映射到主站真实模型，并配置服务商售价';


--
-- Name: COLUMN provider_model_pricings.id; Type: COMMENT;;
--

COMMENT ON COLUMN provider_model_pricings.id IS '服务商模型定价 ID，主键';


--
-- Name: COLUMN provider_model_pricings.provider_id; Type: COMMENT;;
--

COMMENT ON COLUMN provider_model_pricings.provider_id IS '所属服务商 ID，关联 providers.id';


--
-- Name: COLUMN provider_model_pricings.public_model_name; Type: COMMENT;;
--

COMMENT ON COLUMN provider_model_pricings.public_model_name IS '服务商对外展示和用户调用的模型名';


--
-- Name: COLUMN provider_model_pricings.base_model_name; Type: COMMENT;;
--

COMMENT ON COLUMN provider_model_pricings.base_model_name IS '主站真实模型名，实际中继和上游调用使用该模型';


--
-- Name: COLUMN provider_model_pricings.enabled; Type: COMMENT;;
--

COMMENT ON COLUMN provider_model_pricings.enabled IS '是否启用该服务商模型';


--
-- Name: COLUMN provider_model_pricings.pricing_type; Type: COMMENT;;
--

COMMENT ON COLUMN provider_model_pricings.pricing_type IS '定价方式：ratio 表示按比例，delta 表示在主站价格基础上加减';


--
-- Name: COLUMN provider_model_pricings.ratio; Type: COMMENT;;
--

COMMENT ON COLUMN provider_model_pricings.ratio IS '比例定价倍数；pricing_type 为 ratio 时使用，例如 1.2 表示主站价格的 1.2 倍';


--
-- Name: COLUMN provider_model_pricings.delta_model_ratio; Type: COMMENT;;
--

COMMENT ON COLUMN provider_model_pricings.delta_model_ratio IS '按倍率计费模型的加减值；pricing_type 为 delta 且模型按 ratio 计费时使用';


--
-- Name: COLUMN provider_model_pricings.delta_model_price; Type: COMMENT;;
--

COMMENT ON COLUMN provider_model_pricings.delta_model_price IS '按固定价格计费模型的加减金额；pricing_type 为 delta 且模型按 price 计费时使用';


--
-- Name: COLUMN provider_model_pricings.created_at; Type: COMMENT;;
--

COMMENT ON COLUMN provider_model_pricings.created_at IS '创建时间，Unix 秒级时间戳';


--
-- Name: COLUMN provider_model_pricings.updated_at; Type: COMMENT;;
--

COMMENT ON COLUMN provider_model_pricings.updated_at IS '更新时间，Unix 秒级时间戳';


--
-- Name: COLUMN provider_model_pricings.consume_rebate_ratio_level1; Type: COMMENT;;
--

COMMENT ON COLUMN provider_model_pricings.consume_rebate_ratio_level1 IS '一级消费返佣比例，取值 0~100';


--
-- Name: COLUMN provider_model_pricings.consume_rebate_ratio_level2; Type: COMMENT;;
--

COMMENT ON COLUMN provider_model_pricings.consume_rebate_ratio_level2 IS '二级消费返佣比例，取值 0~100';


--
-- Name: COLUMN provider_model_pricings.sync_disabled; Type: COMMENT;;
--

COMMENT ON COLUMN provider_model_pricings.sync_disabled IS '同步软禁用标记：true 表示由自动同步（主站模型下架）禁用；手动保存会清为 false，避免同步误改手动状态';


--
-- Name: provider_model_pricings_id_seq; Type: SEQUENCE;;
--

CREATE SEQUENCE provider_model_pricings_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

--
-- Name: provider_model_pricings_id_seq; Type: SEQUENCE OWNED BY;;
--

ALTER SEQUENCE provider_model_pricings_id_seq OWNED BY provider_model_pricings.id;


--
-- Name: provider_options_id_seq; Type: SEQUENCE;;
--

CREATE SEQUENCE provider_options_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: provider_options; Type: TABLE;;
--

CREATE TABLE provider_options (
    id bigint DEFAULT nextval('provider_options_id_seq'::regclass) NOT NULL,
    provider_id bigint,
    key text NOT NULL,
    value text
);


--
-- Name: TABLE provider_options; Type: COMMENT;;
--

COMMENT ON TABLE provider_options IS '服务商配置表';


--
-- Name: COLUMN provider_options.id; Type: COMMENT;;
--

COMMENT ON COLUMN provider_options.id IS '主键';


--
-- Name: COLUMN provider_options.provider_id; Type: COMMENT;;
--

COMMENT ON COLUMN provider_options.provider_id IS '服务商id';


--
-- Name: COLUMN provider_options.key; Type: COMMENT;;
--

COMMENT ON COLUMN provider_options.key IS '键';


--
-- Name: COLUMN provider_options.value; Type: COMMENT;;
--

COMMENT ON COLUMN provider_options.value IS '值';


--
-- Name: provider_profits; Type: TABLE;;
--

CREATE TABLE provider_profits (
    id bigint NOT NULL,
    provider_id bigint NOT NULL,
    owner_user_id bigint NOT NULL,
    provider_user_id bigint NOT NULL,
    request_id character varying(64) NOT NULL,
    public_model_name character varying(255),
    base_model_name character varying(255),
    provider_user_quota bigint DEFAULT 0,
    base_cost_quota bigint DEFAULT 0,
    paid_quota bigint DEFAULT 0,
    covered_cost_quota bigint DEFAULT 0,
    owner_cost_quota bigint DEFAULT 0,
    profit_quota bigint DEFAULT 0,
    profit_settled boolean DEFAULT false,
    owner_cost_settled boolean DEFAULT false,
    created_at bigint,
    gross_profit_quota bigint DEFAULT 0,
    rebate_quota bigint DEFAULT 0
);


--
-- Name: TABLE provider_profits; Type: COMMENT;;
--

COMMENT ON TABLE provider_profits IS '服务商成功消费后的即时成本和利润入账记录';


--
-- Name: COLUMN provider_profits.id; Type: COMMENT;;
--

COMMENT ON COLUMN provider_profits.id IS '主键ID';


--
-- Name: COLUMN provider_profits.provider_id; Type: COMMENT;;
--

COMMENT ON COLUMN provider_profits.provider_id IS '服务商ID';


--
-- Name: COLUMN provider_profits.owner_user_id; Type: COMMENT;;
--

COMMENT ON COLUMN provider_profits.owner_user_id IS '服务商主账号用户ID';


--
-- Name: COLUMN provider_profits.provider_user_id; Type: COMMENT;;
--

COMMENT ON COLUMN provider_profits.provider_user_id IS '服务商站点下发起调用的用户ID';


--
-- Name: COLUMN provider_profits.request_id; Type: COMMENT;;
--

COMMENT ON COLUMN provider_profits.request_id IS '请求ID，用于保证同一次调用只结算一次';


--
-- Name: COLUMN provider_profits.public_model_name; Type: COMMENT;;
--

COMMENT ON COLUMN provider_profits.public_model_name IS '服务商对外展示和售卖的模型名称';


--
-- Name: COLUMN provider_profits.base_model_name; Type: COMMENT;;
--

COMMENT ON COLUMN provider_profits.base_model_name IS '实际调用主站的基础模型名称';


--
-- Name: COLUMN provider_profits.provider_user_quota; Type: COMMENT;;
--

COMMENT ON COLUMN provider_profits.provider_user_quota IS '服务商用户本次应扣额度，按服务商定价计算';


--
-- Name: COLUMN provider_profits.base_cost_quota; Type: COMMENT;;
--

COMMENT ON COLUMN provider_profits.base_cost_quota IS '主站原价成本额度';


--
-- Name: COLUMN provider_profits.paid_quota; Type: COMMENT;;
--

COMMENT ON COLUMN provider_profits.paid_quota IS '服务商用户本次实际消耗的充值余额额度，不含奖励余额和订阅额度';


--
-- Name: COLUMN provider_profits.covered_cost_quota; Type: COMMENT;;
--

COMMENT ON COLUMN provider_profits.covered_cost_quota IS '充值余额已覆盖的主站成本额度';


--
-- Name: COLUMN provider_profits.owner_cost_quota; Type: COMMENT;;
--

COMMENT ON COLUMN provider_profits.owner_cost_quota IS '还需要服务商主账号承担的成本额度';


--
-- Name: COLUMN provider_profits.profit_quota; Type: COMMENT;;
--

COMMENT ON COLUMN provider_profits.profit_quota IS '本次即时入账给服务商主账号的利润额度';


--
-- Name: COLUMN provider_profits.profit_settled; Type: COMMENT;;
--

COMMENT ON COLUMN provider_profits.profit_settled IS '服务商利润是否已入账';


--
-- Name: COLUMN provider_profits.owner_cost_settled; Type: COMMENT;;
--

COMMENT ON COLUMN provider_profits.owner_cost_settled IS '服务商主账号成本是否已扣除';


--
-- Name: COLUMN provider_profits.created_at; Type: COMMENT;;
--

COMMENT ON COLUMN provider_profits.created_at IS '创建时间戳';


--
-- Name: COLUMN provider_profits.gross_profit_quota; Type: COMMENT;;
--

COMMENT ON COLUMN provider_profits.gross_profit_quota IS '分佣前毛利润';


--
-- Name: COLUMN provider_profits.rebate_quota; Type: COMMENT;;
--

COMMENT ON COLUMN provider_profits.rebate_quota IS '一级 + 二级总分佣';


--
-- Name: provider_profits_id_seq; Type: SEQUENCE;;
--

CREATE SEQUENCE provider_profits_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: provider_profits_id_seq; Type: SEQUENCE OWNED BY;;
--

ALTER SEQUENCE provider_profits_id_seq OWNED BY provider_profits.id;


--
-- Name: provider_reward_configs; Type: TABLE;;
--

CREATE TABLE provider_reward_configs (
    id integer NOT NULL,
    provider_id bigint NOT NULL,
    quota_for_new_user bigint DEFAULT 0,
    quota_for_inviter bigint DEFAULT 0,
    quota_for_invitee bigint DEFAULT 0,
    checkin_enabled boolean DEFAULT false,
    checkin_min_quota bigint DEFAULT 0,
    checkin_max_quota bigint DEFAULT 0,
    invite_topup_rebate_ratio numeric(10,6) DEFAULT 0,
    invite_consume_rebate_ratio_level2 numeric(10,6) DEFAULT 0,
    created_at bigint,
    updated_at bigint
);


--
-- Name: TABLE provider_reward_configs; Type: COMMENT;;
--

COMMENT ON TABLE provider_reward_configs IS '服务商奖励策略配置表';


--
-- Name: COLUMN provider_reward_configs.id; Type: COMMENT;;
--

COMMENT ON COLUMN provider_reward_configs.id IS '主键 ID';


--
-- Name: COLUMN provider_reward_configs.provider_id; Type: COMMENT;;
--

COMMENT ON COLUMN provider_reward_configs.provider_id IS '服务商 ID';


--
-- Name: COLUMN provider_reward_configs.quota_for_new_user; Type: COMMENT;;
--

COMMENT ON COLUMN provider_reward_configs.quota_for_new_user IS '新用户注册赠送额度';


--
-- Name: COLUMN provider_reward_configs.quota_for_inviter; Type: COMMENT;;
--

COMMENT ON COLUMN provider_reward_configs.quota_for_inviter IS '邀请人注册奖励额度';


--
-- Name: COLUMN provider_reward_configs.quota_for_invitee; Type: COMMENT;;
--

COMMENT ON COLUMN provider_reward_configs.quota_for_invitee IS '被邀请人注册奖励额度';


--
-- Name: COLUMN provider_reward_configs.checkin_enabled; Type: COMMENT;;
--

COMMENT ON COLUMN provider_reward_configs.checkin_enabled IS '是否启用签到奖励';


--
-- Name: COLUMN provider_reward_configs.checkin_min_quota; Type: COMMENT;;
--

COMMENT ON COLUMN provider_reward_configs.checkin_min_quota IS '签到奖励最小额度';


--
-- Name: COLUMN provider_reward_configs.checkin_max_quota; Type: COMMENT;;
--

COMMENT ON COLUMN provider_reward_configs.checkin_max_quota IS '签到奖励最大额度';


--
-- Name: COLUMN provider_reward_configs.invite_topup_rebate_ratio; Type: COMMENT;;
--

COMMENT ON COLUMN provider_reward_configs.invite_topup_rebate_ratio IS '邀请充值返利比例';


--
-- Name: COLUMN provider_reward_configs.invite_consume_rebate_ratio_level2; Type: COMMENT;;
--

COMMENT ON COLUMN provider_reward_configs.invite_consume_rebate_ratio_level2 IS '二级邀请消费返利比例';


--
-- Name: COLUMN provider_reward_configs.created_at; Type: COMMENT;;
--

COMMENT ON COLUMN provider_reward_configs.created_at IS '创建时间戳';


--
-- Name: COLUMN provider_reward_configs.updated_at; Type: COMMENT;;
--

COMMENT ON COLUMN provider_reward_configs.updated_at IS '更新时间戳';


--
-- Name: provider_reward_configs_id_seq; Type: SEQUENCE;;
--

CREATE SEQUENCE provider_reward_configs_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

--
-- Name: provider_reward_configs_id_seq; Type: SEQUENCE OWNED BY;;
--

ALTER SEQUENCE provider_reward_configs_id_seq OWNED BY provider_reward_configs.id;


--
-- Name: provider_withdraw_id_seq; Type: SEQUENCE;;
--

CREATE SEQUENCE provider_withdraw_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

--
-- Name: provider_withdraw; Type: TABLE;;
--

CREATE TABLE provider_withdraw (
    id bigint DEFAULT nextval('provider_withdraw_id_seq'::regclass) NOT NULL,
    provider_id bigint NOT NULL,
    amount numeric(18,8) DEFAULT 0,
    currency character varying(20),
    usd_amount numeric(18,8) DEFAULT 0,
    cny_amount numeric(18,8) DEFAULT 0,
    usd_to_cny_rate numeric(18,8) DEFAULT 0,
    status bigint DEFAULT 0,
    created_at bigint DEFAULT 0,
    updated_at bigint DEFAULT 0,
    provider_name text
);

--
-- Name: TABLE provider_withdraw; Type: COMMENT;;
--

COMMENT ON TABLE provider_withdraw IS '服务商提现表';


--
-- Name: COLUMN provider_withdraw.id; Type: COMMENT;;
--

COMMENT ON COLUMN provider_withdraw.id IS '主键';


--
-- Name: COLUMN provider_withdraw.provider_id; Type: COMMENT;;
--

COMMENT ON COLUMN provider_withdraw.provider_id IS '服务商id';


--
-- Name: COLUMN provider_withdraw.amount; Type: COMMENT;;
--

COMMENT ON COLUMN provider_withdraw.amount IS '金额';


--
-- Name: COLUMN provider_withdraw.currency; Type: COMMENT;;
--

COMMENT ON COLUMN provider_withdraw.currency IS '货币';


--
-- Name: COLUMN provider_withdraw.usd_amount; Type: COMMENT;;
--

COMMENT ON COLUMN provider_withdraw.usd_amount IS '美元金额';


--
-- Name: COLUMN provider_withdraw.cny_amount; Type: COMMENT;;
--

COMMENT ON COLUMN provider_withdraw.cny_amount IS '人民币金额';


--
-- Name: COLUMN provider_withdraw.usd_to_cny_rate; Type: COMMENT;;
--

COMMENT ON COLUMN provider_withdraw.usd_to_cny_rate IS '美元到人民币汇率';


--
-- Name: COLUMN provider_withdraw.status; Type: COMMENT;;
--

COMMENT ON COLUMN provider_withdraw.status IS '状态: 1-审核中, 2-审核通过, 3-已拒绝, 4-取消申请';


--
-- Name: COLUMN provider_withdraw.created_at; Type: COMMENT;;
--

COMMENT ON COLUMN provider_withdraw.created_at IS '创建时间';


--
-- Name: COLUMN provider_withdraw.updated_at; Type: COMMENT;;
--

COMMENT ON COLUMN provider_withdraw.updated_at IS '更新时间';


--
-- Name: providers; Type: TABLE;;
--

CREATE TABLE providers (
    id bigint NOT NULL,
    owner_user_id bigint NOT NULL,
    name character varying(128) NOT NULL,
    status bigint DEFAULT 1,
    created_at bigint,
    updated_at bigint
);


--
-- Name: TABLE providers; Type: COMMENT;;
--

COMMENT ON TABLE providers IS '服务商主表：每一条记录代表一个独立服务商，不支持下级服务商';


--
-- Name: COLUMN providers.id; Type: COMMENT;;
--

COMMENT ON COLUMN providers.id IS '服务商 ID，主键';


--
-- Name: COLUMN providers.owner_user_id; Type: COMMENT;;
--

COMMENT ON COLUMN providers.owner_user_id IS '服务商归属的主站用户 ID；服务商调用主站模型时从该用户余额扣除成本';


--
-- Name: COLUMN providers.name; Type: COMMENT;;
--

COMMENT ON COLUMN providers.name IS '服务商名称，用于后台识别和默认展示';


--
-- Name: COLUMN providers.status; Type: COMMENT;;
--

COMMENT ON COLUMN providers.status IS '服务商状态：1 启用，0 禁用';


--
-- Name: COLUMN providers.created_at; Type: COMMENT;;
--

COMMENT ON COLUMN providers.created_at IS '创建时间，Unix 秒级时间戳';


--
-- Name: COLUMN providers.updated_at; Type: COMMENT;;
--

COMMENT ON COLUMN providers.updated_at IS '更新时间，Unix 秒级时间戳';


--
-- Name: providers_id_seq; Type: SEQUENCE;;
--

CREATE SEQUENCE providers_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE providers_id_seq OWNED BY providers.id;



CREATE TABLE quota_data (
    id bigint NOT NULL,
    user_id bigint,
    username character varying(64) DEFAULT ''::character varying,
    model_name character varying(64) DEFAULT ''::character varying,
    created_at bigint,
    token_used bigint DEFAULT 0,
    count bigint DEFAULT 0,
    quota bigint DEFAULT 0
);


CREATE SEQUENCE quota_data_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;



ALTER SEQUENCE quota_data_id_seq OWNED BY quota_data.id;



CREATE TABLE redemptions (
    id bigint NOT NULL,
    user_id bigint,
    key character(32),
    status bigint DEFAULT 1,
    name text,
    quota bigint DEFAULT 100,
    created_time bigint,
    redeemed_time bigint,
    used_user_id bigint,
    deleted_at timestamp with time zone,
    expired_time bigint,
    provider_id bigint DEFAULT 0
);

--
-- Name: COLUMN redemptions.provider_id; Type: COMMENT;;
--

COMMENT ON COLUMN redemptions.provider_id IS '所属服务商 ID，0 表示主站兑换码';


--
-- Name: redemptions_id_seq; Type: SEQUENCE;;
--

CREATE SEQUENCE redemptions_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE redemptions_id_seq OWNED BY redemptions.id;


CREATE TABLE reward_records (
    id integer NOT NULL,
    provider_id bigint DEFAULT 0 NOT NULL,
    user_id bigint NOT NULL,
    source_type character varying(32) NOT NULL,
    source_id bigint DEFAULT 0 NOT NULL,
    quota bigint NOT NULL,
    description character varying(255) DEFAULT ''::character varying,
    created_at bigint
);

--
-- Name: TABLE reward_records; Type: COMMENT;;
--

COMMENT ON TABLE reward_records IS '服务商维度奖励流水表';


--
-- Name: COLUMN reward_records.id; Type: COMMENT;;
--

COMMENT ON COLUMN reward_records.id IS '主键 ID';


--
-- Name: COLUMN reward_records.provider_id; Type: COMMENT;;
--

COMMENT ON COLUMN reward_records.provider_id IS '服务商 ID';


--
-- Name: COLUMN reward_records.user_id; Type: COMMENT;;
--

COMMENT ON COLUMN reward_records.user_id IS '奖励接收用户 ID';


--
-- Name: COLUMN reward_records.source_type; Type: COMMENT;;
--

COMMENT ON COLUMN reward_records.source_type IS '奖励来源类型：新用户、邀请人奖励、被邀请人奖励、签到、兑换码、消费返利、充值返利';


--
-- Name: COLUMN reward_records.source_id; Type: COMMENT;;
--

COMMENT ON COLUMN reward_records.source_id IS '来源业务记录 ID';


--
-- Name: COLUMN reward_records.quota; Type: COMMENT;;
--

COMMENT ON COLUMN reward_records.quota IS '奖励额度';


--
-- Name: COLUMN reward_records.description; Type: COMMENT;;
--

COMMENT ON COLUMN reward_records.description IS '奖励说明';


--
-- Name: COLUMN reward_records.created_at; Type: COMMENT;;
--

COMMENT ON COLUMN reward_records.created_at IS '创建时间戳';


--
-- Name: reward_records_id_seq; Type: SEQUENCE;;
--

CREATE SEQUENCE reward_records_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE reward_records_id_seq OWNED BY reward_records.id;


CREATE TABLE service_configs (
    id bigint NOT NULL,
    merchant_pid character varying(64) DEFAULT ''::character varying NOT NULL,
    merchant_key character varying(128) DEFAULT ''::character varying NOT NULL,
    wechat_app_id character varying(64) DEFAULT ''::character varying NOT NULL,
    wechat_mch_id character varying(64) DEFAULT ''::character varying NOT NULL,
    wechat_mch_serial_no character varying(128) DEFAULT ''::character varying NOT NULL,
    wechat_private_key_path character varying(512) DEFAULT ''::character varying NOT NULL,
    wechat_apiv3_key character varying(64) DEFAULT ''::character varying NOT NULL,
    wechat_notify_url character varying(512) DEFAULT ''::character varying NOT NULL,
    alipay_app_id character varying(64) DEFAULT ''::character varying NOT NULL,
    alipay_private_key text DEFAULT ''::text NOT NULL,
    alipay_public_key text DEFAULT ''::text NOT NULL,
    alipay_notify_url character varying(512) DEFAULT ''::character varying NOT NULL,
    alipay_is_prod boolean DEFAULT false NOT NULL,
    usdt_enabled boolean DEFAULT false NOT NULL,
    usdt_trc20_address character varying(64) DEFAULT ''::character varying NOT NULL,
    usdt_trongrid_api_key character varying(128) DEFAULT ''::character varying NOT NULL,
    usdt_cny_rate double precision DEFAULT 0.137 NOT NULL,
    usdt_poll_interval_sec bigint DEFAULT 15 NOT NULL,
    usdt_expiry_minutes bigint DEFAULT 30 NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    merchant_p_id character varying(64) DEFAULT ''::character varying NOT NULL,
    wechat_serial_no character varying(64),
    wechat_cert_path character varying(255),
    wechat_public_key_path character varying(255)
);

--
-- Name: COLUMN service_configs.wechat_serial_no; Type: COMMENT;;
--

COMMENT ON COLUMN service_configs.wechat_serial_no IS '微信支付平台证书序列号；公钥模式下填写微信支付公钥ID';


--
-- Name: COLUMN service_configs.wechat_cert_path; Type: COMMENT;;
--

COMMENT ON COLUMN service_configs.wechat_cert_path IS '微信支付平台证书路径';

COMMENT ON COLUMN service_configs.wechat_public_key_path IS '微信支付公钥路径，公钥模式下使用；当 wechat_cert_path 为空时读取该字段';
--
-- Name: service_configs_id_seq; Type: SEQUENCE;;
--

CREATE SEQUENCE service_configs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: service_configs_id_seq; Type: SEQUENCE OWNED BY;;
--

ALTER SEQUENCE service_configs_id_seq OWNED BY service_configs.id;


CREATE TABLE setups (
    id bigint NOT NULL,
    version character varying(50) NOT NULL,
    initialized_at bigint NOT NULL
);


CREATE SEQUENCE setups_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE setups_id_seq OWNED BY setups.id;


CREATE TABLE subscription_orders (
    id bigint NOT NULL,
    user_id bigint,
    plan_id bigint,
    money numeric,
    trade_no character varying(255),
    payment_method character varying(50),
    status text,
    create_time bigint,
    complete_time bigint,
    provider_payload text,
    currency character varying(10) DEFAULT ''::character varying,
    original_money numeric(18,6) DEFAULT 0 NOT NULL,
    payment_provider character varying(50) DEFAULT ''::character varying
);


CREATE SEQUENCE subscription_orders_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE subscription_orders_id_seq OWNED BY subscription_orders.id;



CREATE TABLE subscription_plans (
    id bigint NOT NULL,
    title character varying(128) NOT NULL,
    subtitle character varying(255) DEFAULT ''::character varying,
    price_amount numeric(10,6) DEFAULT 0.000000 NOT NULL,
    currency character varying(8) DEFAULT 'USD'::character varying NOT NULL,
    duration_unit character varying(16) DEFAULT 'month'::character varying NOT NULL,
    duration_value bigint DEFAULT 1 NOT NULL,
    custom_seconds bigint DEFAULT 0 NOT NULL,
    enabled boolean DEFAULT true,
    sort_order bigint DEFAULT 0,
    stripe_price_id character varying(128) DEFAULT ''::character varying,
    creem_product_id character varying(128) DEFAULT ''::character varying,
    max_purchase_per_user bigint DEFAULT 0,
    upgrade_group character varying(64) DEFAULT ''::character varying,
    total_amount bigint DEFAULT 0 NOT NULL,
    quota_reset_period character varying(16) DEFAULT 'never'::character varying,
    quota_reset_custom_seconds bigint DEFAULT 0,
    created_at bigint,
    updated_at bigint,
    stripe_price_cny_id character varying(128) DEFAULT ''::character varying,
    waffo_pancake_product_id character varying(128) DEFAULT ''::character varying,
    allow_purchase integer NOT NULL DEFAULT 1,
    model_limits text NOT NULL DEFAULT '',
    provider_id bigint NOT NULL DEFAULT 0
);


COMMENT ON COLUMN subscription_plans.provider_id
    IS '订阅套餐归属服务商 ID，0 表示主站套餐，>0 表示服务商私有套餐';


CREATE SEQUENCE subscription_plans_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: subscription_plans_id_seq; Type: SEQUENCE OWNED BY;;
--

ALTER SEQUENCE subscription_plans_id_seq OWNED BY subscription_plans.id;


--
-- Name: subscription_pre_consume_records; Type: TABLE;;
--

CREATE TABLE subscription_pre_consume_records (
    id bigint NOT NULL,
    request_id character varying(64),
    user_id bigint,
    user_subscription_id bigint,
    pre_consumed bigint DEFAULT 0 NOT NULL,
    status character varying(32),
    created_at bigint,
    updated_at bigint
);


CREATE SEQUENCE subscription_pre_consume_records_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE subscription_pre_consume_records_id_seq OWNED BY subscription_pre_consume_records.id;



CREATE TABLE tasks (
    id bigint NOT NULL,
    created_at bigint,
    updated_at bigint,
    task_id character varying(191),
    platform character varying(30),
    user_id bigint,
    "group" character varying(50),
    channel_id bigint,
    quota bigint,
    action character varying(40),
    status character varying(20),
    fail_reason text,
    submit_time bigint,
    start_time bigint,
    finish_time bigint,
    progress character varying(20),
    properties json,
    private_data json,
    data json
);

CREATE SEQUENCE tasks_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

--
-- Name: tasks_id_seq; Type: SEQUENCE OWNED BY;;
--

ALTER SEQUENCE tasks_id_seq OWNED BY tasks.id;




CREATE TABLE timezone_currency_map (
    timezone character varying(64) NOT NULL,
    currency character varying(3) NOT NULL,
    updated_at timestamp with time zone DEFAULT now()
);



CREATE TABLE tokens (
    id bigint NOT NULL,
    user_id bigint,
    key character varying(128),
    status bigint DEFAULT 1,
    name text,
    created_time bigint,
    accessed_time bigint,
    expired_time bigint DEFAULT '-1'::integer,
    remain_quota bigint DEFAULT 0,
    unlimited_quota boolean,
    model_limits_enabled boolean,
    model_limits text DEFAULT ''::character varying,
    allow_ips text DEFAULT ''::text,
    used_quota bigint DEFAULT 0,
    "group" text DEFAULT ''::text,
    cross_group_retry boolean,
    deleted_at timestamp with time zone,
    provider_id bigint DEFAULT 0,
    total_token_used bigint DEFAULT 0 NOT NULL
);


--
-- Name: COLUMN tokens.provider_id; Type: COMMENT;;
--

COMMENT ON COLUMN tokens.provider_id IS '令牌所属服务商 ID；必须与当前访问域名解析出的 provider_id 一致';


--
-- Name: tokens_id_seq; Type: SEQUENCE;;
--

CREATE SEQUENCE tokens_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

--
-- Name: tokens_id_seq; Type: SEQUENCE OWNED BY;;
--

ALTER SEQUENCE tokens_id_seq OWNED BY tokens.id;


--
-- Name: top_ups; Type: TABLE;;
--

CREATE TABLE top_ups (
    id bigint NOT NULL,
    user_id bigint,
    amount bigint,
    money numeric,
    trade_no character varying(255),
    payment_method character varying(50),
    create_time bigint,
    complete_time bigint,
    status text,
    biz_type character varying(32) DEFAULT 'payment'::character varying,
    source_id bigint DEFAULT 0,
    currency character varying(10) DEFAULT ''::character varying,
    original_money numeric(18,6) DEFAULT 0 NOT NULL,
    provider_id bigint DEFAULT 0,
    payment_provider character varying(50) DEFAULT ''::character varying
);


--
-- Name: COLUMN top_ups.provider_id; Type: COMMENT;;
--

COMMENT ON COLUMN top_ups.provider_id IS '充值订单所属服务商 ID；用于把充值金额加到对应服务商用户余额';


--
-- Name: top_ups_id_seq; Type: SEQUENCE;;
--

CREATE SEQUENCE top_ups_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

--
-- Name: top_ups_id_seq; Type: SEQUENCE OWNED BY;;
--

ALTER SEQUENCE top_ups_id_seq OWNED BY top_ups.id;


--
-- Name: topup_rebates; Type: TABLE;;
--

CREATE TABLE topup_rebates (
    id bigint NOT NULL,
    inviter_id bigint,
    invitee_id bigint,
    topup_id bigint,
    trade_no character varying(255),
    payment_method character varying(50),
    source_money numeric,
    source_quota bigint,
    rebate_ratio numeric,
    rebate_quota bigint,
    created_at bigint,
    money numeric,
    status text,
    provider_id bigint DEFAULT 0
);

--
-- Name: TABLE topup_rebates; Type: COMMENT;;
--

COMMENT ON TABLE topup_rebates IS '邀请充值返利记录表';


--
-- Name: COLUMN topup_rebates.provider_id; Type: COMMENT;;
--

COMMENT ON COLUMN topup_rebates.provider_id IS '所属服务商 ID，0 表示主站充值返利记录';


--
-- Name: topup_rebates_id_seq; Type: SEQUENCE;;
--

CREATE SEQUENCE topup_rebates_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

--
-- Name: topup_rebates_id_seq; Type: SEQUENCE OWNED BY;;
--

ALTER SEQUENCE topup_rebates_id_seq OWNED BY topup_rebates.id;


--
-- Name: two_fa_backup_codes; Type: TABLE;;
--

CREATE TABLE two_fa_backup_codes (
    id bigint NOT NULL,
    user_id bigint NOT NULL,
    code_hash character varying(255) NOT NULL,
    is_used boolean,
    used_at timestamp with time zone,
    created_at timestamp with time zone,
    deleted_at timestamp with time zone
);

--
-- Name: two_fa_backup_codes_id_seq; Type: SEQUENCE;;
--

CREATE SEQUENCE two_fa_backup_codes_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

--
-- Name: two_fa_backup_codes_id_seq; Type: SEQUENCE OWNED BY;;
--

ALTER SEQUENCE two_fa_backup_codes_id_seq OWNED BY two_fa_backup_codes.id;


--
-- Name: two_fas; Type: TABLE;;
--

CREATE TABLE two_fas (
    id bigint NOT NULL,
    user_id bigint NOT NULL,
    secret character varying(255) NOT NULL,
    is_enabled boolean,
    failed_attempts bigint DEFAULT 0,
    locked_until timestamp with time zone,
    last_used_at timestamp with time zone,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone
);


CREATE SEQUENCE two_fas_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE two_fas_id_seq OWNED BY two_fas.id;



CREATE TABLE user_oauth_bindings (
    id bigint NOT NULL,
    user_id bigint NOT NULL,
    provider_id bigint NOT NULL,
    provider_user_id character varying(256) NOT NULL,
    created_at timestamp with time zone
);

CREATE SEQUENCE user_oauth_bindings_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE user_oauth_bindings_id_seq OWNED BY user_oauth_bindings.id;


CREATE TABLE user_subscriptions (
    id bigint NOT NULL,
    user_id bigint,
    plan_id bigint,
    amount_total bigint DEFAULT 0 NOT NULL,
    amount_used bigint DEFAULT 0 NOT NULL,
    start_time bigint,
    end_time bigint,
    status character varying(32),
    source character varying(32) DEFAULT 'order'::character varying,
    last_reset_time bigint DEFAULT 0,
    next_reset_time bigint DEFAULT 0,
    upgrade_group character varying(64) DEFAULT ''::character varying,
    prev_user_group character varying(64) DEFAULT ''::character varying,
    created_at bigint,
    updated_at bigint
);

CREATE SEQUENCE user_subscriptions_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE user_subscriptions_id_seq OWNED BY user_subscriptions.id;



CREATE TABLE users (
    id bigint NOT NULL,
    username text,
    password text NOT NULL,
    display_name text,
    role bigint DEFAULT 1,
    status bigint DEFAULT 1,
    email text,
    github_id text,
    discord_id text,
    oidc_id text,
    wechat_id text,
    telegram_id text,
    access_token character(32),
    quota bigint DEFAULT 0,
    used_quota bigint DEFAULT 0,
    request_count bigint DEFAULT 0,
    "group" character varying(64) DEFAULT 'default'::character varying,
    aff_code character varying(32),
    aff_count bigint DEFAULT 0,
    aff_quota bigint DEFAULT 0,
    aff_history bigint DEFAULT 0,
    inviter_id bigint,
    deleted_at timestamp with time zone,
    linux_do_id text,
    setting text,
    remark character varying(255),
    stripe_customer character varying(64),
    created_at bigint,
    phone_country_code character varying(16),
    phone_number character varying(32),
    timezone character varying(64),
    avatar character varying(2048),
    signup_source character varying(64),
    reward_quota bigint DEFAULT 0,
    provider_id bigint DEFAULT 0,
    total_token_used bigint DEFAULT 0 NOT NULL,
    invite_consume_rebate_enabled bigint DEFAULT 0
);

COMMENT ON COLUMN users.phone_country_code IS '手机号国家区号（E.164），如 +86';


--
-- Name: COLUMN users.phone_number; Type: COMMENT;;
--

COMMENT ON COLUMN users.phone_number IS '手机号本地号码，不含国家区号，如 13800000000';


--
-- Name: COLUMN users.timezone; Type: COMMENT;;
--

COMMENT ON COLUMN users.timezone IS '时区标识（IANA），如 Asia/Shanghai';


--
-- Name: COLUMN users.avatar; Type: COMMENT;;
--

COMMENT ON COLUMN users.avatar IS '头像';


--
-- Name: COLUMN users.signup_source; Type: COMMENT;;
--

COMMENT ON COLUMN users.signup_source IS '注册来源';


--
-- Name: COLUMN users.reward_quota; Type: COMMENT;;
--

COMMENT ON COLUMN users.reward_quota IS '奖励所得额度：注册奖励、邀请奖励、签到、兑换、消费返利等系统奖励累计剩余额度';


--
-- Name: COLUMN users.provider_id; Type: COMMENT;;
--

COMMENT ON COLUMN users.provider_id IS '所属服务商 ID，0 表示主站用户';


--
-- Name: COLUMN users.invite_consume_rebate_enabled; Type: COMMENT;;
--

COMMENT ON COLUMN users.invite_consume_rebate_enabled IS '0：无法领取邀请消费返利，1：可领取邀请消费返利';


--
-- Name: users_id_seq; Type: SEQUENCE;;
--

CREATE SEQUENCE users_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE users_id_seq OWNED BY users.id;


--
-- Name: vendors; Type: TABLE;;
--

CREATE TABLE vendors (
    id bigint NOT NULL,
    name character varying(128) NOT NULL,
    description text,
    icon character varying(128),
    status bigint DEFAULT 1,
    created_time bigint,
    updated_time bigint,
    deleted_at timestamp with time zone
);

--
-- Name: vendors_id_seq; Type: SEQUENCE;;
--

CREATE SEQUENCE vendors_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

--
-- Name: vendors_id_seq; Type: SEQUENCE OWNED BY;;
--

ALTER SEQUENCE vendors_id_seq OWNED BY vendors.id;


--
-- Name: version_log_id_seq; Type: SEQUENCE;;
--

CREATE SEQUENCE version_log_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


CREATE TABLE version_log (
    id bigint DEFAULT nextval('version_log_id_seq'::regclass) NOT NULL,
    version character varying(64),
    log text,
    created_at bigint DEFAULT 0,
    updated_at bigint DEFAULT 0
);

--
-- Name: TABLE version_log; Type: COMMENT;;
--

COMMENT ON TABLE version_log IS '更新日志表';


--
-- Name: COLUMN version_log.id; Type: COMMENT;;
--

COMMENT ON COLUMN version_log.id IS '主键';


--
-- Name: COLUMN version_log.version; Type: COMMENT;;
--

COMMENT ON COLUMN version_log.version IS '版本号';


--
-- Name: COLUMN version_log.log; Type: COMMENT;;
--

COMMENT ON COLUMN version_log.log IS '日志内容';


--
-- Name: COLUMN version_log.created_at; Type: COMMENT;;
--

COMMENT ON COLUMN version_log.created_at IS '创建时间';


--
-- Name: COLUMN version_log.updated_at; Type: COMMENT;;
--

COMMENT ON COLUMN version_log.updated_at IS '更新时间';


--
-- Name: admin_sessions id; Type: DEFAULT;;
--

ALTER TABLE ONLY admin_sessions ALTER COLUMN id SET DEFAULT nextval('admin_sessions_id_seq'::regclass);


--
-- Name: admin_users id; Type: DEFAULT;;
--

ALTER TABLE ONLY admin_users ALTER COLUMN id SET DEFAULT nextval('admin_users_id_seq'::regclass);


--
-- Name: channels id; Type: DEFAULT;;
--

ALTER TABLE ONLY channels ALTER COLUMN id SET DEFAULT nextval('channels_id_seq'::regclass);


--
-- Name: checkins id; Type: DEFAULT;;
--

ALTER TABLE ONLY checkins ALTER COLUMN id SET DEFAULT nextval('checkins_id_seq'::regclass);


--
-- Name: custom_oauth_providers id; Type: DEFAULT;;
--

ALTER TABLE ONLY custom_oauth_providers ALTER COLUMN id SET DEFAULT nextval('custom_oauth_providers_id_seq'::regclass);


--
-- Name: epay_merchants id; Type: DEFAULT;;
--

ALTER TABLE ONLY epay_merchants ALTER COLUMN id SET DEFAULT nextval('epay_merchants_id_seq'::regclass);


--
-- Name: login_audit_logs id; Type: DEFAULT;;
--

ALTER TABLE ONLY login_audit_logs ALTER COLUMN id SET DEFAULT nextval('login_audit_logs_id_seq'::regclass);


--
-- Name: logs id; Type: DEFAULT;;
--

ALTER TABLE ONLY logs ALTER COLUMN id SET DEFAULT nextval('logs_id_seq'::regclass);


--
-- Name: midjourneys id; Type: DEFAULT;;
--

ALTER TABLE ONLY midjourneys ALTER COLUMN id SET DEFAULT nextval('midjourneys_id_seq'::regclass);


--
-- Name: models id; Type: DEFAULT;;
--

ALTER TABLE ONLY models ALTER COLUMN id SET DEFAULT nextval('models_id_seq'::regclass);


--
-- Name: orders id; Type: DEFAULT;;
--

ALTER TABLE ONLY orders ALTER COLUMN id SET DEFAULT nextval('orders_id_seq'::regclass);


--
-- Name: passkey_credentials id; Type: DEFAULT;;
--

ALTER TABLE ONLY passkey_credentials ALTER COLUMN id SET DEFAULT nextval('passkey_credentials_id_seq'::regclass);


--
-- Name: prefill_groups id; Type: DEFAULT;;
--

ALTER TABLE ONLY prefill_groups ALTER COLUMN id SET DEFAULT nextval('prefill_groups_id_seq'::regclass);


--
-- Name: provider_configs id; Type: DEFAULT;;
--

ALTER TABLE ONLY provider_configs ALTER COLUMN id SET DEFAULT nextval('provider_configs_id_seq'::regclass);


--
-- Name: provider_domains id; Type: DEFAULT;;
--

ALTER TABLE ONLY provider_domains ALTER COLUMN id SET DEFAULT nextval('provider_domains_id_seq'::regclass);


--
-- Name: provider_model_pricings id; Type: DEFAULT;;
--

ALTER TABLE ONLY provider_model_pricings ALTER COLUMN id SET DEFAULT nextval('provider_model_pricings_id_seq'::regclass);


--
-- Name: provider_profits id; Type: DEFAULT;;
--

ALTER TABLE ONLY provider_profits ALTER COLUMN id SET DEFAULT nextval('provider_profits_id_seq'::regclass);


--
-- Name: provider_reward_configs id; Type: DEFAULT;;
--

ALTER TABLE ONLY provider_reward_configs ALTER COLUMN id SET DEFAULT nextval('provider_reward_configs_id_seq'::regclass);


--
-- Name: providers id; Type: DEFAULT;;
--

ALTER TABLE ONLY providers ALTER COLUMN id SET DEFAULT nextval('providers_id_seq'::regclass);


--
-- Name: quota_data id; Type: DEFAULT;;
--

ALTER TABLE ONLY quota_data ALTER COLUMN id SET DEFAULT nextval('quota_data_id_seq'::regclass);


--
-- Name: redemptions id; Type: DEFAULT;;
--

ALTER TABLE ONLY redemptions ALTER COLUMN id SET DEFAULT nextval('redemptions_id_seq'::regclass);


--
-- Name: reward_records id; Type: DEFAULT;;
--

ALTER TABLE ONLY reward_records ALTER COLUMN id SET DEFAULT nextval('reward_records_id_seq'::regclass);


--
-- Name: service_configs id; Type: DEFAULT;;
--

ALTER TABLE ONLY service_configs ALTER COLUMN id SET DEFAULT nextval('service_configs_id_seq'::regclass);


--
-- Name: setups id; Type: DEFAULT;;
--

ALTER TABLE ONLY setups ALTER COLUMN id SET DEFAULT nextval('setups_id_seq'::regclass);


--
-- Name: subscription_orders id; Type: DEFAULT;;
--

ALTER TABLE ONLY subscription_orders ALTER COLUMN id SET DEFAULT nextval('subscription_orders_id_seq'::regclass);


--
-- Name: subscription_plans id; Type: DEFAULT;;
--

ALTER TABLE ONLY subscription_plans ALTER COLUMN id SET DEFAULT nextval('subscription_plans_id_seq'::regclass);


--
-- Name: subscription_pre_consume_records id; Type: DEFAULT;;
--

ALTER TABLE ONLY subscription_pre_consume_records ALTER COLUMN id SET DEFAULT nextval('subscription_pre_consume_records_id_seq'::regclass);


--
-- Name: tasks id; Type: DEFAULT;;
--

ALTER TABLE ONLY tasks ALTER COLUMN id SET DEFAULT nextval('tasks_id_seq'::regclass);


--
-- Name: tokens id; Type: DEFAULT;;
--

ALTER TABLE ONLY tokens ALTER COLUMN id SET DEFAULT nextval('tokens_id_seq'::regclass);


--
-- Name: top_ups id; Type: DEFAULT;;
--

ALTER TABLE ONLY top_ups ALTER COLUMN id SET DEFAULT nextval('top_ups_id_seq'::regclass);


--
-- Name: topup_rebates id; Type: DEFAULT;;
--

ALTER TABLE ONLY topup_rebates ALTER COLUMN id SET DEFAULT nextval('topup_rebates_id_seq'::regclass);


--
-- Name: two_fa_backup_codes id; Type: DEFAULT;;
--

ALTER TABLE ONLY two_fa_backup_codes ALTER COLUMN id SET DEFAULT nextval('two_fa_backup_codes_id_seq'::regclass);


--
-- Name: two_fas id; Type: DEFAULT;;
--

ALTER TABLE ONLY two_fas ALTER COLUMN id SET DEFAULT nextval('two_fas_id_seq'::regclass);


--
-- Name: user_oauth_bindings id; Type: DEFAULT;;
--

ALTER TABLE ONLY user_oauth_bindings ALTER COLUMN id SET DEFAULT nextval('user_oauth_bindings_id_seq'::regclass);


--
-- Name: user_subscriptions id; Type: DEFAULT;;
--

ALTER TABLE ONLY user_subscriptions ALTER COLUMN id SET DEFAULT nextval('user_subscriptions_id_seq'::regclass);


--
-- Name: users id; Type: DEFAULT;;
--

ALTER TABLE ONLY users ALTER COLUMN id SET DEFAULT nextval('users_id_seq'::regclass);


--
-- Name: vendors id; Type: DEFAULT;;
--

ALTER TABLE ONLY vendors ALTER COLUMN id SET DEFAULT nextval('vendors_id_seq'::regclass);



INSERT INTO admin_users (id, username, password_hash, password_changed, created_at, updated_at) VALUES (1, 'admin', '$2a$12$EXWCpXd6TVZKe2UTVdMOAO1HCmUbknRZKC3f22AM7VF9Xj5nXjWtK', false, '2026-03-12 18:02:37.145913+08', '2026-03-12 18:02:37.145913+08');




INSERT INTO channels (id, type, key, open_ai_organization, test_model, status, name, weight, created_time, test_time, response_time, base_url, other, balance, balance_updated_time, models, "group", used_quota, model_mapping, status_code_mapping, priority, auto_ban, other_info, tag, setting, param_override, header_override, remark, channel_info, settings) VALUES (1001, 1, '', '', '', 1, 'cliProxyApi_codexcli', 0, 1773987527, 1776335875, 1005, 'https://allrouter.ai', '', 0, 0, 'gpt-5.4,gpt-5.4-mini,gpt-5.2,gpt-5.3-codex', 'Codex CLI', 606842112, '', '', 0, 1, '', '', '{"force_format":false,"thinking_to_content":false,"proxy":"","pass_through_body_enabled":false,"system_prompt":"","system_prompt_override":false}', '', '', NULL, '{"is_multi_key":true,"multi_key_size":1,"multi_key_status_list":null,"multi_key_polling_index":0,"multi_key_mode":"polling"}', '{"allow_service_tier":false,"disable_store":false,"allow_safety_identifier":false,"allow_include_obfuscation":false,"upstream_model_update_check_enabled":false,"upstream_model_update_auto_sync_enabled":false,"upstream_model_update_ignored_models":[],"upstream_model_update_last_detected_models":[],"upstream_model_update_last_check_time":0}');
INSERT INTO channels (id, type, key, open_ai_organization, test_model, status, name, weight, created_time, test_time, response_time, base_url, other, balance, balance_updated_time, models, "group", used_quota, model_mapping, status_code_mapping, priority, auto_ban, other_info, tag, setting, param_override, header_override, remark, channel_info, settings) VALUES (1004, 1, '', '', '', 1, 'cliProxyApi_geminicli', 0, 1773987527, 1773995072, 977, 'https://allrouter.ai', '', 0, 0, 'gemini-3.1-pro-preview,gemini-3-pro-preview', 'codex CLI专用', 17387, '', '', 0, 1, '', '', '{"force_format":false,"thinking_to_content":false,"proxy":"","pass_through_body_enabled":false,"system_prompt":"","system_prompt_override":false}', '', NULL, NULL, '{"is_multi_key":true,"multi_key_size":1,"multi_key_status_list":null,"multi_key_polling_index":0,"multi_key_mode":"polling"}', '{"allow_service_tier":false,"disable_store":false,"allow_safety_identifier":false,"allow_include_obfuscation":false,"upstream_model_update_check_enabled":false,"upstream_model_update_auto_sync_enabled":false,"upstream_model_update_ignored_models":[],"upstream_model_update_last_detected_models":[],"upstream_model_update_last_check_time":0}');
INSERT INTO channels (id, type, key, open_ai_organization, test_model, status, name, weight, created_time, test_time, response_time, base_url, other, balance, balance_updated_time, models, "group", used_quota, model_mapping, status_code_mapping, priority, auto_ban, other_info, tag, setting, param_override, header_override, remark, channel_info, settings) VALUES (1002, 1, '', '', '', 1, 'cliProxyApi_claudecodecli', 0, 1773987527, 1773995072, 977, 'https://allrouter.ai', '', 0, 0, 'gpt-5.4,gpt-5', 'codex CLI专用', 17387, '', '', 0, 1, '', '', '{"force_format":false,"thinking_to_content":false,"proxy":"","pass_through_body_enabled":false,"system_prompt":"","system_prompt_override":false}', '', NULL, NULL, '{"is_multi_key":true,"multi_key_size":1,"multi_key_status_list":null,"multi_key_polling_index":0,"multi_key_mode":"polling"}', '{"allow_service_tier":false,"disable_store":false,"allow_safety_identifier":false,"allow_include_obfuscation":false,"upstream_model_update_check_enabled":false,"upstream_model_update_auto_sync_enabled":false,"upstream_model_update_ignored_models":[],"upstream_model_update_last_detected_models":[],"upstream_model_update_last_check_time":0}');
INSERT INTO channels (id, type, key, open_ai_organization, test_model, status, name, weight, created_time, test_time, response_time, base_url, other, balance, balance_updated_time, models, "group", used_quota, model_mapping, status_code_mapping, priority, auto_ban, other_info, tag, setting, param_override, header_override, remark, channel_info, settings) VALUES (1003, 1, '', '', '', 1, 'cliProxyApi_antigravitycli', 0, 1773987527, 1773995072, 977, 'https://allrouter.ai', '', 0, 0, 'gpt-5.4,gpt-5', 'codex CLI专用', 17387, '', '', 0, 1, '', '', '{"force_format":false,"thinking_to_content":false,"proxy":"","pass_through_body_enabled":false,"system_prompt":"","system_prompt_override":false}', '', NULL, NULL, '{"is_multi_key":true,"multi_key_size":1,"multi_key_status_list":null,"multi_key_polling_index":0,"multi_key_mode":"polling"}', '{"allow_service_tier":false,"disable_store":false,"allow_safety_identifier":false,"allow_include_obfuscation":false,"upstream_model_update_check_enabled":false,"upstream_model_update_auto_sync_enabled":false,"upstream_model_update_ignored_models":[],"upstream_model_update_last_detected_models":[],"upstream_model_update_last_check_time":0}');
INSERT INTO channels (id, type, key, open_ai_organization, test_model, status, name, weight, created_time, test_time, response_time, base_url, other, balance, balance_updated_time, models, "group", used_quota, model_mapping, status_code_mapping, priority, auto_ban, other_info, tag, setting, param_override, header_override, remark, channel_info, settings) VALUES (1005, 1, '', '', '', 1, 'cliProxyApi_kimicli', 0, 1773987527, 1773995072, 977, 'https://allrouter.ai', '', 0, 0, 'gpt-5.4,gpt-5', 'codex CLI专用', 17387, '', '', 0, 1, '', '', '{"force_format":false,"thinking_to_content":false,"proxy":"","pass_through_body_enabled":false,"system_prompt":"","system_prompt_override":false}', '', NULL, NULL, '{"is_multi_key":true,"multi_key_size":1,"multi_key_status_list":null,"multi_key_polling_index":0,"multi_key_mode":"polling"}', '{"allow_service_tier":false,"disable_store":false,"allow_safety_identifier":false,"allow_include_obfuscation":false,"upstream_model_update_check_enabled":false,"upstream_model_update_auto_sync_enabled":false,"upstream_model_update_ignored_models":[],"upstream_model_update_last_detected_models":[],"upstream_model_update_last_check_time":0}');
INSERT INTO channels (id, type, key, open_ai_organization, test_model, status, name, weight, created_time, test_time, response_time, base_url, other, balance, balance_updated_time, models, "group", used_quota, model_mapping, status_code_mapping, priority, auto_ban, other_info, tag, setting, param_override, header_override, remark, channel_info, settings) VALUES (1007, 1, '', '', '', 1, 'cliProxyApi_iflowcli', 0, 1773987527, 1773995072, 977, 'https://allrouter.ai', '', 0, 0, 'gpt-5.4,gpt-5', 'codex CLI专用', 17387, '', '', 0, 1, '', '', '{"force_format":false,"thinking_to_content":false,"proxy":"","pass_through_body_enabled":false,"system_prompt":"","system_prompt_override":false}', '', NULL, NULL, '{"is_multi_key":true,"multi_key_size":1,"multi_key_status_list":null,"multi_key_polling_index":0,"multi_key_mode":"polling"}', '{"allow_service_tier":false,"disable_store":false,"allow_safety_identifier":false,"allow_include_obfuscation":false,"upstream_model_update_check_enabled":false,"upstream_model_update_auto_sync_enabled":false,"upstream_model_update_ignored_models":[],"upstream_model_update_last_detected_models":[],"upstream_model_update_last_check_time":0}');


INSERT INTO models (id, model_name, description, icon, tags, vendor_id, endpoints, status, sync_official, created_time, updated_time, deleted_at, name_rule, description_i18n, features_i18n) VALUES (2, 'Doubao-1.8', '', '', '', 2, '{
  "ep-20251219142058-9g5cq": "Doubao-1.8"
}', 1, 1, 1771911373, 1771912172, '2026-02-24 13:50:14.35909+08', 0, NULL, NULL);
INSERT INTO models (id, model_name, description, icon, tags, vendor_id, endpoints, status, sync_official, created_time, updated_time, deleted_at, name_rule, description_i18n, features_i18n) VALUES (3, 'doubao-1.8', '', '', '', 2, '', 1, 1, 1771912198, 1771912662, '2026-02-24 13:58:03.253988+08', 2, NULL, NULL);
INSERT INTO models (id, model_name, description, icon, tags, vendor_id, endpoints, status, sync_official, created_time, updated_time, deleted_at, name_rule, description_i18n, features_i18n) VALUES (6, 'Doubao-1.8', '', '', '', 0, '', 1, 1, 1772256169, 1772256169, '2026-02-28 13:22:53.587714+08', 0, NULL, NULL);
INSERT INTO models (id, model_name, description, icon, tags, vendor_id, endpoints, status, sync_official, created_time, updated_time, deleted_at, name_rule, description_i18n, features_i18n) VALUES (9, 'MiniMax-M2.5', '', 'Minimax', '', 0, '{
  "MinMax": "MinMax"
}', 0, 1, 1772531909, 1772532003, NULL, 0, NULL, NULL);
INSERT INTO models (id, model_name, description, icon, tags, vendor_id, endpoints, status, sync_official, created_time, updated_time, deleted_at, name_rule, description_i18n, features_i18n) VALUES (5, 'DeepSeek-V3.2', '', '', '', 3, '{
  "deepseek": "deepseek"
}', 0, 1, 1772250841, 1772257876, NULL, 0, NULL, NULL);
INSERT INTO models (id, model_name, description, icon, tags, vendor_id, endpoints, status, sync_official, created_time, updated_time, deleted_at, name_rule, description_i18n, features_i18n) VALUES (4, 'ep-20251219142058-9g5cq', '', '', '', 2, '{
  "字节跳动": "字节跳动"
}', 0, 1, 1771912723, 1771912819, NULL, 0, NULL, NULL);
INSERT INTO models (id, model_name, description, icon, tags, vendor_id, endpoints, status, sync_official, created_time, updated_time, deleted_at, name_rule, description_i18n, features_i18n) VALUES (8, 'claude-sonnet-4-6', '适用于复杂代码开发、代码重构、长链路代码推理与企业级 AI 编程场景，支持多工具协同开发。', '', '', 4, '', 1, 1, 1772414195, 1779783458, NULL, 0, NULL, NULL);
INSERT INTO models (id, model_name, description, icon, tags, vendor_id, endpoints, status, sync_official, created_time, updated_time, deleted_at, name_rule, description_i18n, features_i18n) VALUES (7, 'claude-opus-4-6', '适用于复杂代码开发、代码重构、长链路代码推理与企业级 AI 编程场景，支持多工具协同开发。', '', '', 4, '{
  "anthropic": "anthropic"
}', 1, 1, 1772257783, 1779782176, NULL, 0, NULL, NULL);
INSERT INTO models (id, model_name, description, icon, tags, vendor_id, endpoints, status, sync_official, created_time, updated_time, deleted_at, name_rule, description_i18n, features_i18n) VALUES (11, 'google/gemma-4-26b-a4b-it:free', '', '', '', 13, '["openai"]', 1, 1, 1775619037, 1775619081, NULL, 0, NULL, NULL);
INSERT INTO models (id, model_name, description, icon, tags, vendor_id, endpoints, status, sync_official, created_time, updated_time, deleted_at, name_rule, description_i18n, features_i18n) VALUES (12, 'google/gemma-4-31b-it:free', '', '', '', 13, '', 1, 1, 1775619101, 1775619101, NULL, 0, NULL, NULL);
INSERT INTO models (id, model_name, description, icon, tags, vendor_id, endpoints, status, sync_official, created_time, updated_time, deleted_at, name_rule, description_i18n, features_i18n) VALUES (1, 'gpt-4', '适用于复杂代码开发、代码推理与企业级 AI 编程场景，可构建高性能 AI 开发工作流。', 'OpenAI', '', 1, '[]', 1, 1, 1771906008, 1779782220, NULL, 0, NULL, NULL);
INSERT INTO models (id, model_name, description, icon, tags, vendor_id, endpoints, status, sync_official, created_time, updated_time, deleted_at, name_rule, description_i18n, features_i18n) VALUES (19, 'deepseek-v4-flash', '适用于高并发 AI 对话、实时交互与企业级 Agent 场景，适合与 Hermes、OpenClaw 构建低延迟推理系统。', '', '', 3, '{
  "deepseek": "deepseek"
}', 1, 1, 1779782334, 1779782999, '2026-05-26 16:15:17.68708+08', 0, NULL, NULL);
INSERT INTO models (id, model_name, description, icon, tags, vendor_id, endpoints, status, sync_official, created_time, updated_time, deleted_at, name_rule, description_i18n, features_i18n) VALUES (15, 'qwen3.5-plus', '', '', '', 8, '', 1, 1, 1775704992, 1775704992, '2026-04-09 12:30:57.816434+08', 0, NULL, NULL);
INSERT INTO models (id, model_name, description, icon, tags, vendor_id, endpoints, status, sync_official, created_time, updated_time, deleted_at, name_rule, description_i18n, features_i18n) VALUES (10, 'gemma-4-31b-it', 'Google最新Gemma开源模型系列，Fedimoss推理优化支持
Google Gemma Open Source Series Models, Inference Optimization Powered by Fedimoss AI', '', 'Google开源模型，Fedimoss推理优化', 12, '["openai"]', 1, 1, 1775618986, 1776413658, NULL, 0, NULL, NULL);
INSERT INTO models (id, model_name, description, icon, tags, vendor_id, endpoints, status, sync_official, created_time, updated_time, deleted_at, name_rule, description_i18n, features_i18n) VALUES (18, 'gpt-image-2', '适用于 AI 图片生成、广告设计、视觉创作与数字内容生产场景，可用于多模态创意工作流。', '', '', 5, '{
  "OpenAI": {
    "path": "/v1/images/generations",
    "method": "POST"
  }
}', 1, 1, 1778638020, 1779782055, '2026-05-26 16:15:21.526558+08', 0, NULL, NULL);
INSERT INTO models (id, model_name, description, icon, tags, vendor_id, endpoints, status, sync_official, created_time, updated_time, deleted_at, name_rule, description_i18n, features_i18n) VALUES (16, 'minimax-m2.7', 'Fedimoss推理优化支持
Inference Optimization Powered by Fedimoss AI', 'Minimax', '', 12, '[]', 1, 1, 1776736800, 1776853719, NULL, 0, NULL, NULL);
INSERT INTO models (id, model_name, description, icon, tags, vendor_id, endpoints, status, sync_official, created_time, updated_time, deleted_at, name_rule, description_i18n, features_i18n) VALUES (27, 'deepseek-v4-pro', '适用于复杂推理、代码生成与企业 AI Agent 场景，可结合飞书龙虾与 MCP 工具实现自动化任务执行。', '', '', 3, '{
  "deepseek": "deepseek"
}', 1, 1, 1779782918, 1779782975, '2026-05-26 16:14:51.039072+08', 0, NULL, NULL);
INSERT INTO models (id, model_name, description, icon, tags, vendor_id, endpoints, status, sync_official, created_time, updated_time, deleted_at, name_rule, description_i18n, features_i18n) VALUES (24, 'gpt-5.3-codex', '适用于代码生成、代码补全、自动化开发与 DevOps 场景，适合与 IDE、OpenClaw 等开发工具深度集成。', '', '', 5, '{
  "Openai": "Openai"
}', 1, 1, 1779782443, 1779783084, '2026-05-26 16:14:58.048719+08', 0, NULL, NULL);
INSERT INTO models (id, model_name, description, icon, tags, vendor_id, endpoints, status, sync_official, created_time, updated_time, deleted_at, name_rule, description_i18n, features_i18n) VALUES (28, 'deepseek-v4-flash', '适用于高并发 AI 对话、实时交互与企业级 Agent 场景，适合与 Hermes、OpenClaw 构建低延迟推理系统。', '', '', 3, '', 1, 1, 1779785128, 1779785128, NULL, 0, NULL, NULL);
INSERT INTO models (id, model_name, description, icon, tags, vendor_id, endpoints, status, sync_official, created_time, updated_time, deleted_at, name_rule, description_i18n, features_i18n) VALUES (14, 'anthropic/claude-sonnet-4.6', '适用于代码开发、代码补全、自动化编程与 AI 编程助手场景。', '', '', 13, '["openai"]', 1, 1, 1775624723, 1779782123, NULL, 0, NULL, NULL);
INSERT INTO models (id, model_name, description, icon, tags, vendor_id, endpoints, status, sync_official, created_time, updated_time, deleted_at, name_rule, description_i18n, features_i18n) VALUES (13, 'qwen/qwen3.6-plus', '适用于中文 AI Agent、企业办公与智能客服场景，可结合飞书龙虾与 OpenClaw 构建中文自动化工作流。', '', '', 13, '["openai"]', 1, 1, 1775624606, 1779782133, NULL, 0, NULL, NULL);
INSERT INTO models (id, model_name, description, icon, tags, vendor_id, endpoints, status, sync_official, created_time, updated_time, deleted_at, name_rule, description_i18n, features_i18n) VALUES (17, 'qwen3.6-plus', '适用于中文 AI Agent、企业办公与智能客服场景，可结合飞书龙虾与 OpenClaw 构建中文自动化工作流。', 'Qwen.Color', '', 14, '[]', 1, 1, 1777338265, 1779782095, '2026-05-26 16:15:24.017037+08', 0, NULL, NULL);
INSERT INTO models (id, model_name, description, icon, tags, vendor_id, endpoints, status, sync_official, created_time, updated_time, deleted_at, name_rule, description_i18n, features_i18n) VALUES (29, 'claude-opus-4-6-thinking', '适用于复杂代码推理、算法分析与多步骤开发任务。', '', '', 4, '', 1, 1, 1779785182, 1779785182, NULL, 0, NULL, NULL);
INSERT INTO models (id, model_name, description, icon, tags, vendor_id, endpoints, status, sync_official, created_time, updated_time, deleted_at, name_rule, description_i18n, features_i18n) VALUES (23, 'claude-opus-4-7', '适用于大型代码工程、复杂开发任务与 AI 编程助手场景，支持长上下文代码理解与多工具协同。', '', '', 4, '{
  "anthropic": "anthropic"
}', 1, 1, 1779782436, 1779783054, '2026-05-26 16:15:00.768051+08', 0, NULL, NULL);
INSERT INTO models (id, model_name, description, icon, tags, vendor_id, endpoints, status, sync_official, created_time, updated_time, deleted_at, name_rule, description_i18n, features_i18n) VALUES (26, 'gpt-5.2', '适用于代码开发、自动化编程、AI 编程助手与复杂开发工作流场景，可结合 Hermes、OpenClaw 与 IDE 工具实现智能研发。', '', '', 5, '{
  "Openai": "Openai"
}', 1, 1, 1779782906, 1779783076, '2026-05-26 16:14:53.463111+08', 0, NULL, NULL);
INSERT INTO models (id, model_name, description, icon, tags, vendor_id, endpoints, status, sync_official, created_time, updated_time, deleted_at, name_rule, description_i18n, features_i18n) VALUES (25, 'gemini-3.5-flash', '适用于前端代码开发、实时聊天、AI 搜索与移动端 AI 应用场景，适合构建高并发轻量化 AI 服务。', '', '', 6, '["openai"]', 1, 1, 1779782454, 1779782648, '2026-05-26 16:14:55.961209+08', 0, NULL, NULL);
INSERT INTO models (id, model_name, description, icon, tags, vendor_id, endpoints, status, sync_official, created_time, updated_time, deleted_at, name_rule, description_i18n, features_i18n) VALUES (22, 'gpt-5.4', '适用于复杂代码开发、代码推理与企业级 AI 编程场景，可构建高性能 AI 开发工作流。', '', '', 5, '{
  "Openai": "Openai"
}', 1, 1, 1779782418, 1779783089, '2026-05-26 16:15:05.968667+08', 0, NULL, NULL);
INSERT INTO models (id, model_name, description, icon, tags, vendor_id, endpoints, status, sync_official, created_time, updated_time, deleted_at, name_rule, description_i18n, features_i18n) VALUES (21, 'gpt-5.4-mini', '适用于轻量代码开发、智能代码辅助与中小企业 AI 编程场景，兼顾性能与部署成本。', '', '', 5, '{
  "Openai": "Openai"
}', 1, 1, 1779782409, 1779783071, '2026-05-26 16:15:10.864772+08', 0, NULL, NULL);
INSERT INTO models (id, model_name, description, icon, tags, vendor_id, endpoints, status, sync_official, created_time, updated_time, deleted_at, name_rule, description_i18n, features_i18n) VALUES (20, 'claude-opus-4-6-thinking', '适用于复杂代码推理、算法分析与多步骤开发任务。', '', '', 4, '{
  "anthropic": "anthropic"
}', 1, 1, 1779782366, 1779783059, '2026-05-26 16:15:13.697496+08', 0, NULL, NULL);
INSERT INTO models (id, model_name, description, icon, tags, vendor_id, endpoints, status, sync_official, created_time, updated_time, deleted_at, name_rule, description_i18n, features_i18n) VALUES (30, 'gpt-5.4-mini', '适用于轻量代码开发、智能代码辅助与中小企业 AI 编程场景，兼顾性能与部署成本。', '', '', 5, '', 1, 1, 1779785194, 1779785194, NULL, 0, NULL, NULL);
INSERT INTO models (id, model_name, description, icon, tags, vendor_id, endpoints, status, sync_official, created_time, updated_time, deleted_at, name_rule, description_i18n, features_i18n) VALUES (31, 'gpt-5.4', '适用于复杂代码开发、代码推理与企业级 AI 编程场景，可构建高性能 AI 开发工作流。', '', '', 5, '', 1, 1, 1779785205, 1779785205, NULL, 0, NULL, NULL);
INSERT INTO models (id, model_name, description, icon, tags, vendor_id, endpoints, status, sync_official, created_time, updated_time, deleted_at, name_rule, description_i18n, features_i18n) VALUES (32, 'claude-opus-4-7', '适用于大型代码工程、复杂开发任务与 AI 编程助手场景，支持长上下文代码理解与多工具协同。', '', '', 4, '', 1, 1, 1779785337, 1779785337, NULL, 0, NULL, NULL);
INSERT INTO models (id, model_name, description, icon, tags, vendor_id, endpoints, status, sync_official, created_time, updated_time, deleted_at, name_rule, description_i18n, features_i18n) VALUES (33, 'gpt-5.3-codex', '适用于代码生成、代码补全、自动化开发与 DevOps 场景，适合与 IDE、OpenClaw 等开发工具深度集成。', '', '', 5, '', 1, 1, 1779785347, 1779785347, NULL, 0, NULL, NULL);
INSERT INTO models (id, model_name, description, icon, tags, vendor_id, endpoints, status, sync_official, created_time, updated_time, deleted_at, name_rule, description_i18n, features_i18n) VALUES (34, 'gemini-3.5-flash', '适用于前端代码开发、实时聊天、AI 搜索与移动端 AI 应用场景，适合构建高并发轻量化 AI 服务。', '', '', 6, '', 1, 1, 1779785360, 1779785360, NULL, 0, NULL, NULL);
INSERT INTO models (id, model_name, description, icon, tags, vendor_id, endpoints, status, sync_official, created_time, updated_time, deleted_at, name_rule, description_i18n, features_i18n) VALUES (35, 'gemini-3-pro-preview', '适用于多模态分析、长文档处理与复杂 AI 推理场景，可用于企业知识管理与智能办公。', '', '', 6, '', 1, 1, 1779785397, 1779785397, NULL, 0, NULL, NULL);
INSERT INTO models (id, model_name, description, icon, tags, vendor_id, endpoints, status, sync_official, created_time, updated_time, deleted_at, name_rule, description_i18n, features_i18n) VALUES (36, 'gpt-5.2', '适用于代码开发、自动化编程、AI 编程助手与复杂开发工作流场景，可结合 Hermes、OpenClaw 与 IDE 工具实现智能研发。', '', '', 5, '', 1, 1, 1779785408, 1779785408, NULL, 0, NULL, NULL);
INSERT INTO models (id, model_name, description, icon, tags, vendor_id, endpoints, status, sync_official, created_time, updated_time, deleted_at, name_rule, description_i18n, features_i18n) VALUES (37, 'deepseek-v4-pro', '适用于复杂推理、代码生成与企业 AI Agent 场景，可结合飞书龙虾与 MCP 工具实现自动化任务执行。', '', '', 3, '', 1, 1, 1779785419, 1779785419, NULL, 0, NULL, NULL);
INSERT INTO models (id, model_name, description, icon, tags, vendor_id, endpoints, status, sync_official, created_time, updated_time, deleted_at, name_rule, description_i18n, features_i18n) VALUES (38, 'gpt-5.5', '适用于企业级 AI 编程助手、复杂代码推理与自主开发 Agent 场景，可结合 Hermes 与 OpenClaw 实现自动化研发。', '', '', 5, '', 1, 1, 1779785441, 1779785441, NULL, 0, NULL, NULL);
INSERT INTO models (id, model_name, description, icon, tags, vendor_id, endpoints, status, sync_official, created_time, updated_time, deleted_at, name_rule, description_i18n, features_i18n) VALUES (39, 'claude-sonnet-4-6-thinking', '适用于代码开发、任务规划与复杂逻辑推理场景，可结合 IDE 工具提升研发效率。', '', '', 4, '', 1, 1, 1779785453, 1779785453, NULL, 0, NULL, NULL);
INSERT INTO models (id, model_name, description, icon, tags, vendor_id, endpoints, status, sync_official, created_time, updated_time, deleted_at, name_rule, description_i18n, features_i18n) VALUES (40, 'gemini-3.1-pro-preview', '适用于多模态分析、长文档处理与复杂 AI 推理场景，可用于企业知识管理与智能办公。', '', '', 6, '', 1, 1, 1779785468, 1779785468, NULL, 0, NULL, NULL);
INSERT INTO models (id, model_name, description, icon, tags, vendor_id, endpoints, status, sync_official, created_time, updated_time, deleted_at, name_rule, description_i18n, features_i18n) VALUES (41, 'gpt-image-2', '适用于 AI 图片生成、广告设计、视觉创作与数字内容生产场景，可用于多模态创意工作流。', '', '', 5, '', 1, 1, 1779785502, 1779785502, NULL, 0, NULL, NULL);
INSERT INTO models (id, model_name, description, icon, tags, vendor_id, endpoints, status, sync_official, created_time, updated_time, deleted_at, name_rule, description_i18n, features_i18n) VALUES (42, 'claude-opus-4-7-thinking', '适用于复杂代码架构设计、长链路开发任务与自主编程 Agent 场景，可实现持续代码推理与任务规划。', '', '', 4, '', 1, 1, 1779785517, 1779785517, NULL, 0, NULL, NULL);
INSERT INTO models (id, model_name, description, icon, tags, vendor_id, endpoints, status, sync_official, created_time, updated_time, deleted_at, name_rule, description_i18n, features_i18n) VALUES (43, 'claude-opus-4-8', '适用于复杂代码架构设计、长链路开发任务与自主编程 Agent 场景，可实现持续代码推理与任务规划。', '', '', 0, '', 1, 1, 1780370054, 1780370054, NULL, 0, NULL, NULL);


--
-- Data for Name: options; Type: TABLE DATA;;
--

INSERT INTO options (key, value) VALUES ('DemoSiteEnabled', 'false');
INSERT INTO options (key, value) VALUES ('SelfUseModeEnabled', 'false');
INSERT INTO options (key, value) VALUES ('SystemName', 'AllRouter.AI');
INSERT INTO options (key, value) VALUES ('general_setting.docs_link', 'docs');
INSERT INTO options (key, value) VALUES ('general_setting.quota_display_type', 'USD');
INSERT INTO options (key, value) VALUES ('StripePromotionCodesEnabled', 'false');
INSERT INTO options (key, value) VALUES ('EpayKey', 'xxx');
INSERT INTO options (key, value) VALUES ('ModelRequestRateLimitDurationMinutes', '60');
INSERT INTO options (key, value) VALUES ('ModelRequestRateLimitCount', '1000');
INSERT INTO options (key, value) VALUES ('ModelRequestRateLimitSuccessCount', '1000');
INSERT INTO options (key, value) VALUES ('StripeWebhookSecret', 'xxx');
INSERT INTO options (key, value) VALUES ('UserUsableGroups', '{
  "default": "默认分组",
  "svip": "svip分组",
  "vip": "vip分组",
  "OpenRouter": "OpenRouter账号池",
  "Codex CLI": "自建号池，满血，无场景限制",
  "google cli": "gemini模型的使用",
  "Claude Code": "Claude Code专用",
  "img": "gpt生图"
}');
INSERT INTO options (key, value) VALUES ('GroupRatio', '{
  "default": 1,
  "svip": 1,
  "vip": 1,
  "OpenRouter": 3,
  "Codex CLI": 1,
  "google cli": 1,
  "Claude Code": 1,
  "test": 0.01,
  "img": 1
}');
INSERT INTO options (key, value) VALUES ('StripeApiSecret', 'xxx');
INSERT INTO options (key, value) VALUES ('StripePriceId', 'xxx');
INSERT INTO options (key, value) VALUES ('performance_setting.disk_cache_enabled', 'true');
INSERT INTO options (key, value) VALUES ('PayAddress', 'https://allrouter.shengjian.net/epay-api');
INSERT INTO options (key, value) VALUES ('EpayId', '1001');
INSERT INTO options (key, value) VALUES ('Price', '1');
INSERT INTO options (key, value) VALUES ('ModelRatio', '{
  "360GPT_S2_V9": 0.8572,
  "360gpt-pro": 0.8572,
  "360gpt-turbo": 0.0858,
  "360gpt-turbo-responsibility-8k": 0.8572,
  "360gpt2-pro": 0.8572,
  "ada": 10,
  "anthropic/claude-sonnet-4.6": 2.5,
  "babbage": 10,
  "babbage-002": 0.2,
  "bge-large-en": 0.13698630137,
  "bge-large-zh": 0.13698630137,
  "big-pickle": 0.5,
  "BLOOMZ-7B": 0.27397260274,
  "chatglm_lite": 0.1429,
  "chatglm_pro": 0.7143,
  "chatglm_std": 0.3572,
  "chatglm_turbo": 0.3572,
  "chatgpt-4o-latest": 2.5,
  "claude-3-5-haiku-20241022": 0.4,
  "claude-3-5-sonnet-20240620": 1.5,
  "claude-3-5-sonnet-20241022": 1.5,
  "claude-3-7-sonnet-20250219": 1.5,
  "claude-3-7-sonnet-20250219-thinking": 1.5,
  "claude-3-haiku-20240307": 0.125,
  "claude-3-opus-20240229": 7.5,
  "claude-3-sonnet-20240229": 1.5,
  "claude-fable-5": 5,
  "claude-haiku-4-5-20251001": 0.5,
  "claude-haiku-4.5": 0.125,
  "claude-opus-4-1-20250805": 7.5,
  "claude-opus-4-1-20250805-thinking": 7.5,
  "claude-opus-4-20250514": 7.5,
  "claude-opus-4-20250514-thinking": 7.5,
  "claude-opus-4-5-20251101": 2.5,
  "claude-opus-4-5-20251101-thinking": 2.5,
  "claude-opus-4-6": 2.5,
  "claude-opus-4-6-high": 2.5,
  "claude-opus-4-6-low": 2.5,
  "claude-opus-4-6-max": 2.5,
  "claude-opus-4-6-medium": 0.35,
  "claude-opus-4-6-thinking": 0.35,
  "claude-opus-4-7": 2.5,
  "claude-opus-4-7-thinking": 2.5,
  "claude-opus-4-8": 2.5,
  "claude-sonnet-4-20250514": 0.0411,
  "claude-sonnet-4-20250514-thinking": 0.0411,
  "claude-sonnet-4-5-20250929": 1.5,
  "claude-sonnet-4-5-20250929-thinking": 1.5,
  "claude-sonnet-4-6": 1.5,
  "claude-sonnet-4-6-thinking": 1.5,
  "code-davinci-edit-001": 10,
  "command": 0.5,
  "command-light": 0.5,
  "command-light-nightly": 0.5,
  "command-nightly": 0.5,
  "command-r": 0.25,
  "command-r-08-2024": 0.075,
  "command-r-plus": 1.5,
  "command-r-plus-08-2024": 1.25,
  "curie": 10,
  "davinci": 10,
  "davinci-002": 1,
  "deepseek-ai/DeepSeek-R1": 0.8,
  "deepseek-ai/DeepSeek-R1-0528": 0.8,
  "deepseek-ai/DeepSeek-V3-0324": 0.8,
  "deepseek-ai/DeepSeek-V3.1": 0.8,
  "deepseek-chat": 0.135,
  "deepseek-coder": 0.135,
  "deepseek-reasoner": 0.275,
  "DeepSeek-V3.2": 2.5,
  "deepseek-v4-flash": 0.055,
  "deepseek-v4-pro": 0.66,
  "Doubao-1.8": 2.5,
  "embedding_s1_v1": 0.0715,
  "embedding-bert-512-v1": 0.0715,
  "Embedding-V1": 0.13698630137,
  "ep-20251219142058-9g5cq": 2.5,
  "ERNIE-3.5-4K-0205": 0.821917808219,
  "ERNIE-3.5-8K": 0.821917808219,
  "ERNIE-3.5-8K-0205": 1.643835616438,
  "ERNIE-3.5-8K-1222": 0.821917808219,
  "ERNIE-4.0-8K": 8.219178082192,
  "ERNIE-Bot-8K": 1.643835616438,
  "ERNIE-Lite-8K-0308": 0.205479452055,
  "ERNIE-Lite-8K-0922": 0.547945205479,
  "ERNIE-Speed-128K": 0.27397260274,
  "ERNIE-Speed-8K": 0.27397260274,
  "ERNIE-Tiny-8K": 0.068493150685,
  "gemini-1.5-flash-latest": 0.075,
  "gemini-1.5-pro-latest": 1.25,
  "gemini-2.0-flash": 0.05,
  "gemini-2.5-flash": 0.15,
  "gemini-2.5-flash-lite": 0.05,
  "gemini-2.5-flash-lite-preview-06-17": 0.05,
  "gemini-2.5-flash-lite-preview-thinking-*": 0.05,
  "gemini-2.5-flash-preview-04-17": 0.075,
  "gemini-2.5-flash-preview-04-17-nothinking": 0.075,
  "gemini-2.5-flash-preview-04-17-thinking": 0.075,
  "gemini-2.5-flash-preview-05-20": 0.075,
  "gemini-2.5-flash-preview-05-20-nothinking": 0.075,
  "gemini-2.5-flash-preview-05-20-thinking": 0.075,
  "gemini-2.5-flash-thinking-*": 0.075,
  "gemini-2.5-pro": 0.625,
  "gemini-2.5-pro-exp-03-25": 0.625,
  "gemini-2.5-pro-preview-03-25": 0.625,
  "gemini-2.5-pro-thinking-*": 0.625,
  "gemini-3-flash-preview": 0.25,
  "gemini-3-pro-preview": 1,
  "gemini-3.1-pro-high": 0.5,
  "gemini-3.1-pro-low": 0.5,
  "gemini-3.1-pro-preview": 1,
  "gemini-3.1-pro-preview-customtools": 1,
  "gemini-3.5-flash": 0.75,
  "gemini-embedding-001": 0.075,
  "gemini-robotics-er-1.5-preview": 0.15,
  "gemma-4-31b-it": 0.1,
  "glm-3-turbo": 0.3572,
  "glm-4": 7.143,
  "glm-4-0520": 6.849315068493,
  "glm-4-air": 0.068493150685,
  "glm-4-airx": 0.684931506849,
  "glm-4-alltools": 6.849315068493,
  "glm-4-flash": 0,
  "glm-4-long": 0.068493150685,
  "glm-4-plus": 3.424657534247,
  "glm-4.7": 1.5,
  "glm-4v": 3.424657534247,
  "glm-4v-plus": 0.684931506849,
  "glm-5": 1.5,
  "glm-5.1": 0.329,
  "GLM-5.1": 0.49,
  "glm5.1": 1.5,
  "GLM5.1": 0.0005,
  "GLM5.2": 0.7,
  "gpt-3.5-turbo": 0.25,
  "gpt-3.5-turbo-0125": 0.25,
  "gpt-3.5-turbo-0613": 0.75,
  "gpt-3.5-turbo-1106": 0.5,
  "gpt-3.5-turbo-16k": 1.5,
  "gpt-3.5-turbo-16k-0613": 1.5,
  "gpt-3.5-turbo-instruct": 0.75,
  "gpt-4": 15,
  "gpt-4-0125-preview": 5,
  "gpt-4-0613": 15,
  "gpt-4-1106-preview": 5,
  "gpt-4-1106-vision-preview": 5,
  "gpt-4-32k": 30,
  "gpt-4-32k-0613": 30,
  "gpt-4-all": 15,
  "gpt-4-turbo": 5,
  "gpt-4-turbo-2024-04-09": 5,
  "gpt-4-turbo-preview": 5,
  "gpt-4-vision-preview": 5,
  "gpt-4.1": 1,
  "gpt-4.1-2025-04-14": 1,
  "gpt-4.1-mini": 0.2,
  "gpt-4.1-mini-2025-04-14": 0.2,
  "gpt-4.1-nano": 0.05,
  "gpt-4.1-nano-2025-04-14": 0.05,
  "gpt-4.5-preview": 37.5,
  "gpt-4.5-preview-2025-02-27": 37.5,
  "gpt-4o": 1.25,
  "gpt-4o-2024-05-13": 2.5,
  "gpt-4o-2024-08-06": 1.25,
  "gpt-4o-2024-11-20": 1.25,
  "gpt-4o-all": 15,
  "gpt-4o-audio-preview": 1.25,
  "gpt-4o-audio-preview-2024-10-01": 1.25,
  "gpt-4o-gizmo-*": 2.5,
  "gpt-4o-mini": 0.075,
  "gpt-4o-mini-2024-07-18": 0.075,
  "gpt-4o-mini-audio-preview": 0.075,
  "gpt-4o-mini-realtime": 0.15,
  "gpt-4o-mini-realtime-preview": 0.3,
  "gpt-4o-mini-realtime-preview-2024-12-17": 0.3,
  "gpt-4o-realtime": 1.25,
  "gpt-4o-realtime-preview": 2.5,
  "gpt-4o-realtime-preview-2024-10-01": 2.5,
  "gpt-4o-realtime-preview-2024-12-17": 2.5,
  "gpt-5": 0.625,
  "gpt-5-2025-08-07": 0.625,
  "gpt-5-chat-latest": 0.625,
  "gpt-5-codex": 0.625,
  "gpt-5-codex-high": 0.625,
  "gpt-5-codex-low": 0.625,
  "gpt-5-codex-medium": 0.625,
  "gpt-5-codex-mini": 0.125,
  "gpt-5-codex-mini-high": 0.125,
  "gpt-5-codex-mini-medium": 0.125,
  "gpt-5-high": 0.625,
  "gpt-5-medium": 0.625,
  "gpt-5-mini": 0.625,
  "gpt-5-mini-2025-08-07": 0.625,
  "gpt-5-nano": 0.025,
  "gpt-5-nano-2025-08-07": 0.025,
  "gpt-5.1": 0.625,
  "gpt-5.1-codex": 0.625,
  "gpt-5.1-codex-high": 0.625,
  "gpt-5.1-codex-low": 0.625,
  "gpt-5.1-codex-max": 0.625,
  "gpt-5.1-codex-max-high": 0.625,
  "gpt-5.1-codex-max-low": 0.625,
  "gpt-5.1-codex-max-medium": 0.625,
  "gpt-5.1-codex-max-xhigh": 0.625,
  "gpt-5.1-codex-medium": 0.625,
  "gpt-5.1-codex-mini": 0.625,
  "gpt-5.1-codex-mini-high": 0.625,
  "gpt-5.1-codex-mini-medium": 0.625,
  "gpt-5.1-high": 0.625,
  "gpt-5.1-low": 0.625,
  "gpt-5.1-medium": 0.625,
  "gpt-5.2": 0.875,
  "gpt-5.2-codex": 0.875,
  "gpt-5.2-codex-high": 0.875,
  "gpt-5.2-codex-low": 0.875,
  "gpt-5.2-codex-medium": 0.875,
  "gpt-5.2-codex-xhigh": 0.875,
  "gpt-5.2-high": 0.875,
  "gpt-5.2-low": 0.875,
  "gpt-5.2-medium": 0.875,
  "gpt-5.2-xhigh": 0.875,
  "gpt-5.3-codex": 0.875,
  "gpt-5.3-codex-high": 0.875,
  "gpt-5.3-codex-low": 0.875,
  "gpt-5.3-codex-medium": 0.875,
  "gpt-5.3-codex-spark": 2.615,
  "gpt-5.3-codex-spark-high": 2.615,
  "gpt-5.3-codex-spark-low": 2.615,
  "gpt-5.3-codex-spark-medium": 2.615,
  "gpt-5.3-codex-spark-xhigh": 2.615,
  "gpt-5.3-codex-xhigh": 0.875,
  "gpt-5.4": 1.25,
  "gpt-5.4-high": 1.25,
  "gpt-5.4-high-openai-compact": 1.25,
  "gpt-5.4-low": 1.25,
  "gpt-5.4-low-openai-compact": 1.25,
  "gpt-5.4-medium": 1.25,
  "gpt-5.4-medium-openai-compact": 1.25,
  "gpt-5.4-mini": 1.25,
  "gpt-5.4-openai-compact": 1.25,
  "gpt-5.4-xhigh": 1.25,
  "gpt-5.4-xhigh-openai-compact": 1.25,
  "gpt-5.5": 2.5,
  "gpt-5.5-openai-compact": 2.5,
  "gpt-image-1": 2.5,
  "grok-2": 1,
  "grok-2-vision": 1,
  "grok-3": 1.25,
  "grok-3-beta": 1.5,
  "grok-3-deepersearch": 1.5,
  "grok-3-deepsearch": 1.5,
  "grok-3-fast-beta": 2.5,
  "grok-3-mini": 1.25,
  "grok-3-mini-beta": 0.15,
  "grok-3-mini-fast-beta": 0.3,
  "grok-3-reasoner": 1.5,
  "grok-4": 1.5,
  "grok-4-0709": 1.5,
  "grok-4-1-fast-non-reasoning": 0.1,
  "grok-4-1-fast-reasoning": 0.1,
  "grok-4-fast-non-reasoning": 0.1,
  "grok-4-fast-reasoning": 0.1,
  "grok-4.1": 1.5,
  "grok-beta": 2.5,
  "grok-vision-beta": 2.5,
  "hunyuan": 7.143,
  "kimi-k2.5": 1.5,
  "kimi-k2.6": 0.356,
  "llama-3-sonar-large-32k-chat": 0,
  "llama-3-sonar-large-32k-online": 0,
  "llama-3-sonar-small-32k-chat": 0.1,
  "llama-3-sonar-small-32k-online": 0.1,
  "MiniMax-M2.5": 1.5,
  "minimax-m2.7": 0.25,
  "nemotron-3-super": 0.5,
  "NousResearch/Hermes-4-405B-FP8": 0.8,
  "o1": 7.5,
  "o1-2024-12-17": 7.5,
  "o1-mini": 0.55,
  "o1-mini-2024-09-12": 0.55,
  "o1-preview": 7.5,
  "o1-preview-2024-09-12": 7.5,
  "o1-pro": 75,
  "o1-pro-2025-03-19": 75,
  "o3": 1,
  "o3-2025-04-16": 1,
  "o3-deep-research": 5,
  "o3-deep-research-2025-06-26": 5,
  "o3-mini": 0.55,
  "o3-mini-2025-01-31": 0.55,
  "o3-mini-2025-01-31-high": 0.55,
  "o3-mini-2025-01-31-low": 0.55,
  "o3-mini-2025-01-31-medium": 0.55,
  "o3-mini-high": 0.55,
  "o3-mini-low": 0.55,
  "o3-mini-medium": 0.55,
  "o3-pro": 10,
  "o3-pro-2025-06-10": 10,
  "o4-mini": 0.55,
  "o4-mini-2025-04-16": 0.55,
  "o4-mini-deep-research": 1,
  "o4-mini-deep-research-2025-06-26": 1,
  "openai/gpt-oss-120b": 0.5,
  "PaLM-2": 1,
  "qwen-3.7-max": 0.05,
  "qwen-plus": 10,
  "qwen-turbo": 0.8572,
  "Qwen/Qwen3-235B-A22B-Instruct-2507": 0.3,
  "Qwen/Qwen3-235B-A22B-Thinking-2507": 0.6,
  "Qwen/Qwen3-Coder-480B-A35B-Instruct-FP8": 0.8,
  "qwen/qwen3.6-plus": 0.1625,
  "qwen3-coder-next": 1.5,
  "qwen3-max-2026-01-23": 1.5,
  "qwen3.5-35b-a3b": 0.0035,
  "qwen3.5-plus": 0.75,
  "qwen3.6-plus": 0.329,
  "ring-2.6-1t": 0.5,
  "semantic_similarity_s1_v1": 0.0715,
  "SparkDesk-v1.1": 1.2858,
  "SparkDesk-v2.1": 1.2858,
  "SparkDesk-v3.1": 1.2858,
  "SparkDesk-v3.5": 1.2858,
  "SparkDesk-v4.0": 1.2858,
  "tao-8k": 0.13698630137,
  "text-ada-001": 0.2,
  "text-babbage-001": 0.25,
  "text-curie-001": 1,
  "text-davinci-edit-001": 10,
  "text-embedding-004": 0.001,
  "text-embedding-3-large": 0.065,
  "text-embedding-3-small": 0.01,
  "text-embedding-ada-002": 0.05,
  "text-embedding-v1": 0.05,
  "text-moderation-latest": 0.1,
  "text-moderation-stable": 0.1,
  "text-search-ada-doc-001": 10,
  "tts-1": 7.5,
  "tts-1-1106": 7.5,
  "tts-1-hd": 15,
  "tts-1-hd-1106": 15,
  "whisper-1": 15,
  "yi-34b-chat-0205": 0.18,
  "yi-34b-chat-200k": 0.864,
  "yi-large": 1.369863013698,
  "yi-large-preview": 1.369863013698,
  "yi-large-rag": 1.712328767123,
  "yi-large-rag-preview": 1.712328767123,
  "yi-large-turbo": 0.821917808219,
  "yi-medium": 0.171232876713,
  "yi-medium-200k": 0.821917808219,
  "yi-spark": 0.068493150685,
  "yi-vision": 0.410958904109,
  "yi-vl-plus": 0.432,
  "zai-org/GLM-4.5-FP8": 0.8
}');
INSERT INTO options (key, value) VALUES ('Logo', 'https://allrouter.ai/static/logo/400a60553f96c412e91254c749b5424c6ebbb3f005235225477f0831b0bfbe22.svg');
INSERT INTO options (key, value) VALUES ('ModelRequestRateLimitEnabled', 'false');
INSERT INTO options (key, value) VALUES ('CacheRatio', '{
  "claude-3-5-haiku-20241022": 0.05,
  "claude-3-5-sonnet-20240620": 0.1,
  "claude-3-5-sonnet-20241022": 0.1,
  "claude-3-7-sonnet-20250219": 0.1,
  "claude-3-7-sonnet-20250219-thinking": 0.1,
  "claude-3-haiku-20240307": 0.1,
  "claude-3-opus-20240229": 0.1,
  "claude-3-sonnet-20240229": 0.1,
  "claude-fable-5": 0.1,
  "claude-haiku-4-5-20251001": 0.1,
  "claude-opus-4-1-20250805": 0.1,
  "claude-opus-4-1-20250805-thinking": 0.006,
  "claude-opus-4-20250514": 0.2055,
  "claude-opus-4-20250514-thinking": 0.2055,
  "claude-opus-4-5-20251101": 0.0685,
  "claude-opus-4-5-20251101-thinking": 0.0685,
  "claude-opus-4-6": 0.0685,
  "claude-opus-4-6-high": 0.0685,
  "claude-opus-4-6-low": 0.0685,
  "claude-opus-4-6-max": 0.0685,
  "claude-opus-4-6-medium": 0.0685,
  "claude-opus-4-6-thinking": 0.0685,
  "claude-opus-4-7": 0.0685,
  "claude-opus-4-7-thinking": 0.0685,
  "claude-opus-4-8": 0.1,
  "claude-sonnet-4-20250514": 0.0412,
  "claude-sonnet-4-20250514-thinking": 0.0412,
  "claude-sonnet-4-5-20250929": 0.0412,
  "claude-sonnet-4-5-20250929-thinking": 0.0412,
  "claude-sonnet-4-6": 0.0412,
  "claude-sonnet-4-6-thinking": 0.0412,
  "deepseek-chat": 0.25,
  "deepseek-coder": 0.25,
  "deepseek-reasoner": 0.25,
  "deepseek-v4-flash": 0.2,
  "deepseek-v4-pro": 0.083333333333,
  "gemini-3-flash-preview": 0.1,
  "gemini-3-pro-preview": 0.1,
  "gemini-3.1-pro-high": 0.2,
  "gemini-3.1-pro-low": 0.2,
  "gemini-3.1-pro-preview": 0.1,
  "gemini-3.5-flash": 0.1,
  "gpt-4": 0.5,
  "gpt-4.1": 0.25,
  "gpt-4.1-mini": 0.25,
  "gpt-4.1-nano": 0.25,
  "gpt-4.5-preview": 0.5,
  "gpt-4.5-preview-2025-02-27": 0.5,
  "gpt-4o": 0.5,
  "gpt-4o-2024-08-06": 0.5,
  "gpt-4o-2024-11-20": 0.5,
  "gpt-4o-mini": 0.5,
  "gpt-4o-mini-2024-07-18": 0.5,
  "gpt-4o-mini-realtime-preview": 0.5,
  "gpt-4o-realtime-preview": 0.5,
  "gpt-5": 0.1,
  "gpt-5-2025-08-07": 0.1,
  "gpt-5-chat-latest": 0.1,
  "gpt-5-mini": 0.02,
  "gpt-5-mini-2025-08-07": 0.02,
  "gpt-5-nano": 0.1,
  "gpt-5-nano-2025-08-07": 0.1,
  "gpt-5.2": 0.013142857143,
  "gpt-5.2-codex": 0.013142857143,
  "gpt-5.2-codex-high": 0.013142857143,
  "gpt-5.2-codex-low": 0.013142857143,
  "gpt-5.2-codex-medium": 0.013142857143,
  "gpt-5.2-codex-xhigh": 0.013142857143,
  "gpt-5.2-high": 0.013142857143,
  "gpt-5.2-low": 0.013142857143,
  "gpt-5.2-medium": 0.013142857143,
  "gpt-5.2-xhigh": 0.013142857143,
  "gpt-5.3-codex": 0.006571428571,
  "gpt-5.3-codex-high": 0.006571428571,
  "gpt-5.3-codex-low": 0.006571428571,
  "gpt-5.3-codex-medium": 0.006571428571,
  "gpt-5.3-codex-spark-high": 0.002198852772,
  "gpt-5.3-codex-spark-low": 0.002198852772,
  "gpt-5.3-codex-spark-medium": 0.002198852772,
  "gpt-5.3-codex-spark-xhigh": 0.002198852772,
  "gpt-5.3-codex-xhigh": 0.006571428571,
  "gpt-5.4": 0.1,
  "gpt-5.4-high": 0.1,
  "gpt-5.4-high-openai-compact": 0.1,
  "gpt-5.4-low": 0.1,
  "gpt-5.4-low-openai-compact": 0.1,
  "gpt-5.4-medium": 0.1,
  "gpt-5.4-medium-openai-compact": 0.1,
  "gpt-5.4-mini": 0.1,
  "gpt-5.4-openai-compact": 0.1,
  "gpt-5.4-xhigh": 0.1,
  "gpt-5.4-xhigh-openai-compact": 0.1,
  "gpt-5.5": 0.1,
  "gpt-5.5-openai-compact": 0.1,
  "kimi-k2.6": 0.169943820225,
  "o1": 0.5,
  "o1-2024-12-17": 0.5,
  "o1-mini": 0.5,
  "o1-mini-2024-09-12": 0.5,
  "o1-preview": 0.5,
  "o1-preview-2024-09-12": 0.5,
  "o3-mini": 0.5,
  "o3-mini-2025-01-31": 0.5
}');
INSERT INTO options (key, value) VALUES ('ImageRatio', '{
  "gpt-image-1": 2
}');
INSERT INTO options (key, value) VALUES ('AudioRatio', '{
  "gpt-4o-audio-preview": 16,
  "gpt-4o-mini-audio-preview": 0,
  "gpt-4o-mini-realtime-preview": 16.67,
  "gpt-4o-realtime-preview": 8
}');
INSERT INTO options (key, value) VALUES ('AudioCompletionRatio', '{}');
INSERT INTO options (key, value) VALUES ('MinTopUp', '1');
INSERT INTO options (key, value) VALUES ('ServerAddress', 'https://allrouter.ai');
INSERT INTO options (key, value) VALUES ('CustomCallbackAddress', 'https://allrouter.ai');
INSERT INTO options (key, value) VALUES ('USDExchangeRate', '6.82');
INSERT INTO options (key, value) VALUES ('PayMethods', '[
  {
		"color": "rgba(var(--semi-green-5), 1)",
		"name": "微信",
		"type": "wxpay"
	},
  {
		"color": "rgba(var(--semi-green-5), 1)",
		"name": "支付宝",
		"type": "alipay"
	},
	{
		"color": "rgba(var(--semi-green-5), 1)",
		"name": "Stripe",
		"type": "stripe"
	}
]');
INSERT INTO options (key, value) VALUES ('QuotaForNewUser', '0');
INSERT INTO options (key, value) VALUES ('StripeUnitPrice', '1');
INSERT INTO options (key, value) VALUES ('StripeMinTopUp', '1');
INSERT INTO options (key, value) VALUES ('CLIProxyAPIPassword', 'sj@cli@2026');
INSERT INTO options (key, value) VALUES ('QuotaForInviter', '0');
INSERT INTO options (key, value) VALUES ('CLIServerAddress', 'http://172.31.39.126:8317');
INSERT INTO options (key, value) VALUES ('console_setting.api_info', '[{"id":1,"url":"https://allrouter.ai/","description":"稳定","route":"主线路","color":"blue"}]');
INSERT INTO options (key, value) VALUES ('EmailVerificationEnabled', 'true');
INSERT INTO options (key, value) VALUES ('DrawingEnabled', 'true');
INSERT INTO options (key, value) VALUES ('SMTPSSLEnabled', 'true');
INSERT INTO options (key, value) VALUES ('CreateCacheRatio', '{
  "claude-3-5-haiku-20241022": 0.625,
  "claude-3-5-sonnet-20240620": 1.25,
  "claude-3-5-sonnet-20241022": 1.25,
  "claude-3-7-sonnet-20250219": 1.25,
  "claude-3-7-sonnet-20250219-thinking": 1.25,
  "claude-3-haiku-20240307": 1.25,
  "claude-3-opus-20240229": 1.25,
  "claude-3-sonnet-20240229": 1.25,
  "claude-fable-5": 1.25,
  "claude-haiku-4-5-20251001": 1.25,
  "claude-opus-4-1-20250805": 1.25,
  "claude-opus-4-1-20250805-thinking": 0.075,
  "claude-opus-4-20250514": 2.5685,
  "claude-opus-4-20250514-thinking": 2.5685,
  "claude-opus-4-5-20251101": 0.8562,
  "claude-opus-4-5-20251101-thinking": 0.8562,
  "claude-opus-4-6": 0.175,
  "claude-opus-4-6-high": 0.8562,
  "claude-opus-4-6-low": 0.8562,
  "claude-opus-4-6-max": 0.8562,
  "claude-opus-4-6-medium": 0.8562,
  "claude-opus-4-6-thinking": 0.8562,
  "claude-opus-4-7": 0.175,
  "claude-opus-4-7-thinking": 0.175,
  "claude-opus-4-8": 1.25,
  "claude-sonnet-4-20250514": 0.0411,
  "claude-sonnet-4-20250514-thinking": 0.0411,
  "claude-sonnet-4-5-20250929": 0.0411,
  "claude-sonnet-4-5-20250929-thinking": 0.0411,
  "claude-sonnet-4-6": 0.0411,
  "claude-sonnet-4-6-thinking": 0.0411,
  "gemini-3.1-pro-high": 0.2,
  "gemini-3.1-pro-low": 0.2,
  "gemini-3.5-flash": 0.055553333333,
  "gpt-5.2": 0.171257142857,
  "gpt-5.2-codex": 0.171257142857,
  "gpt-5.2-codex-high": 0.171257142857,
  "gpt-5.2-codex-low": 0.171257142857,
  "gpt-5.2-codex-medium": 0.171257142857,
  "gpt-5.2-codex-xhigh": 0.171257142857,
  "gpt-5.2-high": 0.171257142857,
  "gpt-5.2-low": 0.171257142857,
  "gpt-5.2-medium": 0.171257142857,
  "gpt-5.2-xhigh": 0.171257142857,
  "gpt-5.3-codex": 0.171257142857,
  "gpt-5.3-codex-high": 0.171257142857,
  "gpt-5.3-codex-low": 0.171257142857,
  "gpt-5.3-codex-medium": 0.171257142857,
  "gpt-5.3-codex-spark-high": 0.171887189293,
  "gpt-5.3-codex-spark-low": 0.171887189293,
  "gpt-5.3-codex-spark-medium": 0.171887189293,
  "gpt-5.3-codex-spark-xhigh": 0.171887189293,
  "gpt-5.3-codex-xhigh": 0.171257142857,
  "gpt-5.4": 0.17124,
  "gpt-5.4-high": 0.17124,
  "gpt-5.4-high-openai-compact": 0.17124,
  "gpt-5.4-low": 0.171232,
  "gpt-5.4-low-openai-compact": 0.171232,
  "gpt-5.4-medium": 0.171232,
  "gpt-5.4-medium-openai-compact": 0.171232,
  "gpt-5.4-mini": 0.17124,
  "gpt-5.4-openai-compact": 0.171232,
  "gpt-5.4-xhigh": 0.171232,
  "gpt-5.4-xhigh-openai-compact": 0.171232,
  "gpt-5.5": 0.1,
  "gpt-5.5-openai-compact": 0.1
}');
INSERT INTO options (key, value) VALUES ('SMTPFrom', 'support@allrouter.ai');
INSERT INTO options (key, value) VALUES ('SMTPAccount', 'resend');
INSERT INTO options (key, value) VALUES ('SMTPToken', 'xxx');
INSERT INTO options (key, value) VALUES ('SMTPServer', 'smtp.resend.com');
INSERT INTO options (key, value) VALUES ('SMTPPort', '465');
INSERT INTO options (key, value) VALUES ('PasswordRegisterEnabled', 'true');
INSERT INTO options (key, value) VALUES ('QuotaForInvitee', '0');
INSERT INTO options (key, value) VALUES ('checkin_setting.enabled', 'false');
INSERT INTO options (key, value) VALUES ('RetryTimes', '3');
INSERT INTO options (key, value) VALUES ('DefaultUseAutoGroup', 'false');
INSERT INTO options (key, value) VALUES ('AutoGroups', '[]');
INSERT INTO options (key, value) VALUES ('InviteTopupRebateRatio', '10');
INSERT INTO options (key, value) VALUES ('CompletionRatio', '{
  "anthropic/claude-sonnet-4.6": 5,
  "big-pickle": 1,
  "claude-fable-5": 5,
  "claude-haiku-4.5": 5,
  "claude-opus-4-1-20250805-thinking": 5.25,
  "claude-opus-4-20250514-thinking": 5.25,
  "claude-opus-4-6": 5.15,
  "claude-opus-4-6-high": 5.15,
  "claude-opus-4-6-low": 5.15,
  "claude-opus-4-6-max": 5.15,
  "claude-opus-4-6-medium": 5.15,
  "claude-opus-4-6-thinking": 5.15,
  "claude-opus-4-7": 5,
  "claude-opus-4-7-thinking": 5,
  "claude-opus-4-8": 5,
  "claude-sonnet-4-20250514": 2,
  "claude-sonnet-4-5-20250929": 3,
  "claude-sonnet-4-5-20250929-thinking": 0.7,
  "claude-sonnet-4-6": 5.11,
  "claude-sonnet-4-6-thinking": 5.11,
  "deepseek-v4-flash": 2,
  "deepseek-v4-pro": 1.992424242424,
  "gemini-2.5-flash": 8.333333333333,
  "gemini-2.5-flash-lite": 4,
  "gemini-2.5-pro": 8,
  "gemini-3-flash-preview": 6,
  "gemini-3-pro-preview": 6,
  "gemini-3.1-pro-high": 6,
  "gemini-3.1-pro-low": 6,
  "gemini-3.1-pro-preview": 6,
  "gemini-3.1-pro-preview-customtools": 6,
  "gemini-3.5-flash": 6,
  "gemma-4-31b-it": 5,
  "glm-5.1": 3.996960486322,
  "GLM-5.1": 3.142857142857,
  "glm5.1": 5,
  "GLM5.1": 1,
  "GLM5.2": 3.142857142857,
  "gpt-4-all": 2,
  "gpt-4o": 4,
  "gpt-4o-2024-08-06": 4,
  "gpt-4o-2024-11-20": 4,
  "gpt-4o-gizmo-*": 3,
  "gpt-4o-mini": 4,
  "gpt-4o-mini-audio-preview": 4,
  "gpt-4o-mini-realtime": 4,
  "gpt-4o-realtime": 4,
  "gpt-5.4": 8,
  "gpt-5.4-high": 8,
  "gpt-5.4-high-openai-compact": 6,
  "gpt-5.4-low": 8,
  "gpt-5.4-low-openai-compact": 1.2,
  "gpt-5.4-medium": 8,
  "gpt-5.4-medium-openai-compact": 1.2,
  "gpt-5.4-mini": 6,
  "gpt-5.4-openai-compact": 1.2,
  "gpt-5.4-xhigh": 8,
  "gpt-5.4-xhigh-openai-compact": 1.2,
  "gpt-5.5": 6,
  "gpt-image-1": 8,
  "grok-3-deepersearch": 5,
  "grok-3-deepsearch": 5,
  "grok-3-reasoner": 5,
  "grok-4": 5,
  "grok-4-0709": 5,
  "grok-4-1-fast-non-reasoning": 2.5,
  "grok-4-1-fast-reasoning": 2.5,
  "grok-4-fast-non-reasoning": 2.5,
  "grok-4-fast-reasoning": 2.5,
  "grok-4.1": 5,
  "kimi-k2.6": 4.157303370787,
  "minimax-m2.7": 5,
  "nemotron-3-super": 1,
  "qwen-3.7-max": 1,
  "qwen/qwen3.6-plus": 6,
  "qwen3.5-35b-a3b": 1,
  "qwen3.6-plus": 3.996960486322,
  "ring-2.6-1t": 1
}');
INSERT INTO options (key, value) VALUES ('Version', 'V1.0.3');
INSERT INTO options (key, value) VALUES ('CryptoUSDtoTokenRate', '1');
INSERT INTO options (key, value) VALUES ('CryptoCNYtoTokenRate', '0.1471');
INSERT INTO options (key, value) VALUES ('WebPrimaryColor', '#09FEF7');
INSERT INTO options (key, value) VALUES ('WebSecondaryColor', '#BAFF29');
INSERT INTO options (key, value) VALUES ('WebButtonTextColor', '#005C59');
INSERT INTO options (key, value) VALUES ('PreConsumedQuota', '0');
INSERT INTO options (key, value) VALUES ('channel_affinity_setting.rules', '[{"name":"codex cli trace","model_regex":["^gpt-.*$"],"path_regex":["/v1/responses"],"key_sources":[{"type":"gjson","key":"","path":"prompt_cache_key"}],"value_regex":"","ttl_seconds":0,"include_using_group":true,"include_model_name":false,"include_rule_name":true,"skip_retry_on_failure":false,"param_override_template":{"operations":[{"keep_origin":true,"mode":"pass_headers","value":["Originator","Session_id","User-Agent","X-Codex-Beta-Features","X-Codex-Turn-Metadata"]}]}},{"name":"claude cli trace","model_regex":["^claude-.*$"],"path_regex":["/v1/messages"],"key_sources":[{"type":"gjson","key":"","path":"metadata.user_id"}],"value_regex":"","ttl_seconds":0,"include_using_group":true,"include_model_name":false,"include_rule_name":true,"skip_retry_on_failure":false,"param_override_template":{"operations":[{"keep_origin":true,"mode":"pass_headers","value":["X-Stainless-Arch","X-Stainless-Lang","X-Stainless-Os","X-Stainless-Package-Version","X-Stainless-Retry-Count","X-Stainless-Runtime","X-Stainless-Runtime-Version","X-Stainless-Timeout","User-Agent","X-App","Anthropic-Beta","Anthropic-Dangerous-Direct-Browser-Access","Anthropic-Version"]}]}}]');
INSERT INTO options (key, value) VALUES ('CheckSensitiveOnPromptEnabled', 'false');
INSERT INTO options (key, value) VALUES ('CheckSensitiveEnabled', 'false');
INSERT INTO options (key, value) VALUES ('InviteConsumeRebateRatioLevel2', '0');
INSERT INTO options (key, value) VALUES ('ModelPrice', '{
  "black-forest-labs/flux-1.1-pro": 0.04,
  "dall-e-3": 0.04,
  "gemini-2.5-flash-image": 0.2,
  "gemini-3-pro-image-preview": 0.95,
  "gemini-3.1-flash-image-preview": 0.5,
  "gpt-4-gizmo-*": 0.1,
  "gpt-4o-mini-tts": 0.3,
  "gpt-image": 0.01,
  "gpt-image-2": 0.1,
  "imagen-3.0-generate-002": 0.03,
  "mj_blend": 0.1,
  "mj_custom_zoom": 0,
  "mj_describe": 0.05,
  "mj_edits": 0.1,
  "mj_high_variation": 0.1,
  "mj_imagine": 0.1,
  "mj_inpaint": 0,
  "mj_low_variation": 0.1,
  "mj_modal": 0.1,
  "mj_pan": 0.1,
  "mj_reroll": 0.1,
  "mj_shorten": 0.1,
  "mj_upload": 0.05,
  "mj_upscale": 0.05,
  "mj_variation": 0.1,
  "mj_video": 0.8,
  "mj_zoom": 0.1,
  "sora-2": 0.3,
  "sora-2-pro": 0.5,
  "suno_lyrics": 0.01,
  "suno_music": 0.1,
  "swap_face": 0.05,
  "veo-3.1-fast-generate-preview": 1.8,
  "veo-3.1-generate-preview": 1.8
}');



INSERT INTO timezone_currency_map (timezone, currency, updated_at) VALUES ('Asia/Shanghai', 'CNY', '2026-04-27 13:28:21.3974+08');
INSERT INTO timezone_currency_map (timezone, currency, updated_at) VALUES ('Asia/Chongqing', 'CNY', '2026-04-27 13:28:21.3974+08');
INSERT INTO timezone_currency_map (timezone, currency, updated_at) VALUES ('Asia/Harbin', 'CNY', '2026-04-27 13:28:21.3974+08');
INSERT INTO timezone_currency_map (timezone, currency, updated_at) VALUES ('Asia/Urumqi', 'CNY', '2026-04-27 13:28:21.3974+08');
INSERT INTO timezone_currency_map (timezone, currency, updated_at) VALUES ('Asia/Kashgar', 'CNY', '2026-04-27 13:28:21.3974+08');
INSERT INTO timezone_currency_map (timezone, currency, updated_at) VALUES ('Asia/Macau', 'CNY', '2026-04-27 13:28:21.3974+08');
INSERT INTO timezone_currency_map (timezone, currency, updated_at) VALUES ('Asia/Macao', 'CNY', '2026-04-27 13:28:21.3974+08');
INSERT INTO timezone_currency_map (timezone, currency, updated_at) VALUES ('Asia/Hong_Kong', 'CNY', '2026-04-27 13:28:21.3974+08');
INSERT INTO timezone_currency_map (timezone, currency, updated_at) VALUES ('Asia/Taipei', 'CNY', '2026-04-27 13:28:21.3974+08');
INSERT INTO timezone_currency_map (timezone, currency, updated_at) VALUES ('America/New_York', 'USD', '2026-04-27 13:28:36.477662+08');
INSERT INTO timezone_currency_map (timezone, currency, updated_at) VALUES ('America/Chicago', 'USD', '2026-04-27 13:28:36.477662+08');
INSERT INTO timezone_currency_map (timezone, currency, updated_at) VALUES ('America/Denver', 'USD', '2026-04-27 13:28:36.477662+08');
INSERT INTO timezone_currency_map (timezone, currency, updated_at) VALUES ('America/Los_Angeles', 'USD', '2026-04-27 13:28:36.477662+08');
INSERT INTO timezone_currency_map (timezone, currency, updated_at) VALUES ('America/Anchorage', 'USD', '2026-04-27 13:28:36.477662+08');
INSERT INTO timezone_currency_map (timezone, currency, updated_at) VALUES ('America/Adak', 'USD', '2026-04-27 13:28:36.477662+08');
INSERT INTO timezone_currency_map (timezone, currency, updated_at) VALUES ('America/Phoenix', 'USD', '2026-04-27 13:28:36.477662+08');
INSERT INTO timezone_currency_map (timezone, currency, updated_at) VALUES ('America/Detroit', 'USD', '2026-04-27 13:28:36.477662+08');
INSERT INTO timezone_currency_map (timezone, currency, updated_at) VALUES ('America/Indiana/Indianapolis', 'USD', '2026-04-27 13:28:36.477662+08');
INSERT INTO timezone_currency_map (timezone, currency, updated_at) VALUES ('America/Indiana/Knox', 'USD', '2026-04-27 13:28:36.477662+08');
INSERT INTO timezone_currency_map (timezone, currency, updated_at) VALUES ('America/Indiana/Marengo', 'USD', '2026-04-27 13:28:36.477662+08');
INSERT INTO timezone_currency_map (timezone, currency, updated_at) VALUES ('America/Indiana/Petersburg', 'USD', '2026-04-27 13:28:36.477662+08');
INSERT INTO timezone_currency_map (timezone, currency, updated_at) VALUES ('America/Indiana/Tell_City', 'USD', '2026-04-27 13:28:36.477662+08');
INSERT INTO timezone_currency_map (timezone, currency, updated_at) VALUES ('America/Indiana/Vevay', 'USD', '2026-04-27 13:28:36.477662+08');
INSERT INTO timezone_currency_map (timezone, currency, updated_at) VALUES ('America/Indiana/Vincennes', 'USD', '2026-04-27 13:28:36.477662+08');
INSERT INTO timezone_currency_map (timezone, currency, updated_at) VALUES ('America/Indiana/Winamac', 'USD', '2026-04-27 13:28:36.477662+08');
INSERT INTO timezone_currency_map (timezone, currency, updated_at) VALUES ('America/Kentucky/Louisville', 'USD', '2026-04-27 13:28:36.477662+08');
INSERT INTO timezone_currency_map (timezone, currency, updated_at) VALUES ('America/Kentucky/Monticello', 'USD', '2026-04-27 13:28:36.477662+08');
INSERT INTO timezone_currency_map (timezone, currency, updated_at) VALUES ('America/Boise', 'USD', '2026-04-27 13:28:36.477662+08');
INSERT INTO timezone_currency_map (timezone, currency, updated_at) VALUES ('America/Nome', 'USD', '2026-04-27 13:28:36.477662+08');
INSERT INTO timezone_currency_map (timezone, currency, updated_at) VALUES ('America/Sitka', 'USD', '2026-04-27 13:28:36.477662+08');
INSERT INTO timezone_currency_map (timezone, currency, updated_at) VALUES ('America/Juneau', 'USD', '2026-04-27 13:28:36.477662+08');
INSERT INTO timezone_currency_map (timezone, currency, updated_at) VALUES ('America/Metlakatla', 'USD', '2026-04-27 13:28:36.477662+08');
INSERT INTO timezone_currency_map (timezone, currency, updated_at) VALUES ('America/Yakutat', 'USD', '2026-04-27 13:28:36.477662+08');
INSERT INTO timezone_currency_map (timezone, currency, updated_at) VALUES ('America/North_Dakota/Beulah', 'USD', '2026-04-27 13:28:36.477662+08');
INSERT INTO timezone_currency_map (timezone, currency, updated_at) VALUES ('America/North_Dakota/Center', 'USD', '2026-04-27 13:28:36.477662+08');
INSERT INTO timezone_currency_map (timezone, currency, updated_at) VALUES ('America/North_Dakota/New_Salem', 'USD', '2026-04-27 13:28:36.477662+08');
INSERT INTO timezone_currency_map (timezone, currency, updated_at) VALUES ('America/Menominee', 'USD', '2026-04-27 13:28:36.477662+08');
INSERT INTO timezone_currency_map (timezone, currency, updated_at) VALUES ('America/Shiprock', 'USD', '2026-04-27 13:28:36.477662+08');
INSERT INTO timezone_currency_map (timezone, currency, updated_at) VALUES ('America/Fort_Wayne', 'USD', '2026-04-27 13:28:36.477662+08');
INSERT INTO timezone_currency_map (timezone, currency, updated_at) VALUES ('America/Atikokan', 'USD', '2026-04-27 13:28:36.477662+08');
INSERT INTO timezone_currency_map (timezone, currency, updated_at) VALUES ('America/Coral_Harbour', 'USD', '2026-04-27 13:28:36.477662+08');
INSERT INTO timezone_currency_map (timezone, currency, updated_at) VALUES ('America/Nipigon', 'USD', '2026-04-27 13:28:36.477662+08');
INSERT INTO timezone_currency_map (timezone, currency, updated_at) VALUES ('America/Pangnirtung', 'USD', '2026-04-27 13:28:36.477662+08');
INSERT INTO timezone_currency_map (timezone, currency, updated_at) VALUES ('America/Rainy_River', 'USD', '2026-04-27 13:28:36.477662+08');
INSERT INTO timezone_currency_map (timezone, currency, updated_at) VALUES ('America/Rankin_Inlet', 'USD', '2026-04-27 13:28:36.477662+08');
INSERT INTO timezone_currency_map (timezone, currency, updated_at) VALUES ('America/Resolute', 'USD', '2026-04-27 13:28:36.477662+08');
INSERT INTO timezone_currency_map (timezone, currency, updated_at) VALUES ('America/Thunder_Bay', 'USD', '2026-04-27 13:28:36.477662+08');
INSERT INTO timezone_currency_map (timezone, currency, updated_at) VALUES ('America/Winnipeg', 'USD', '2026-04-27 13:28:36.477662+08');
INSERT INTO timezone_currency_map (timezone, currency, updated_at) VALUES ('America/Swift_Current', 'USD', '2026-04-27 13:28:36.477662+08');
INSERT INTO timezone_currency_map (timezone, currency, updated_at) VALUES ('America/Regina', 'USD', '2026-04-27 13:28:36.477662+08');
INSERT INTO timezone_currency_map (timezone, currency, updated_at) VALUES ('America/Cambridge_Bay', 'USD', '2026-04-27 13:28:36.477662+08');
INSERT INTO timezone_currency_map (timezone, currency, updated_at) VALUES ('America/Edmonton', 'USD', '2026-04-27 13:28:36.477662+08');
INSERT INTO timezone_currency_map (timezone, currency, updated_at) VALUES ('America/Creston', 'USD', '2026-04-27 13:28:36.477662+08');
INSERT INTO timezone_currency_map (timezone, currency, updated_at) VALUES ('America/Dawson_Creek', 'USD', '2026-04-27 13:28:36.477662+08');
INSERT INTO timezone_currency_map (timezone, currency, updated_at) VALUES ('America/Fort_Nelson', 'USD', '2026-04-27 13:28:36.477662+08');
INSERT INTO timezone_currency_map (timezone, currency, updated_at) VALUES ('America/Inuvik', 'USD', '2026-04-27 13:28:36.477662+08');
INSERT INTO timezone_currency_map (timezone, currency, updated_at) VALUES ('America/Whitehorse', 'USD', '2026-04-27 13:28:36.477662+08');
INSERT INTO timezone_currency_map (timezone, currency, updated_at) VALUES ('America/Dawson', 'USD', '2026-04-27 13:28:36.477662+08');
INSERT INTO timezone_currency_map (timezone, currency, updated_at) VALUES ('America/Yellowknife', 'USD', '2026-04-27 13:28:36.477662+08');
INSERT INTO timezone_currency_map (timezone, currency, updated_at) VALUES ('America/Iqaluit', 'USD', '2026-04-27 13:28:36.477662+08');



INSERT INTO users (id, username, password, display_name, role, status, email, github_id, discord_id, oidc_id, wechat_id, telegram_id, access_token, quota, used_quota, request_count, "group", aff_code, aff_count, aff_quota, aff_history, inviter_id, deleted_at, linux_do_id, setting, remark, stripe_customer, created_at, phone_country_code, phone_number, timezone, avatar, signup_source) VALUES (1, 'admin', '$2a$10$0x7Vi0I3FyptefsyuA2C.etw1adn5X/fpwrMY0iOjPjMQi83QYYAS', 'Root User', 100, 1, '', '', '', '', '', '', NULL, 100000000, 0, 0, 'default', '2hWc', 1, 0, 0, 0, NULL, '', '{"gotify_priority":0,"language":"zh-CN"}', '', 'cus_U7BvsXV7SS3lGt', 0, '+86', '', 'Asia/Shanghai', '/assets/logo-white-D3lyOuka.svg', '0');



INSERT INTO vendors (id, name, description, icon, status, created_time, updated_time, deleted_at) VALUES (1, 'openai', '', 'OpenAI.Avatar.type={''platform''}', 1, 1771905962, 1771905962, NULL);
INSERT INTO vendors (id, name, description, icon, status, created_time, updated_time, deleted_at) VALUES (2, '字节跳动', '', 'Doubao.Color', 1, 1771911191, 1771911191, NULL);
INSERT INTO vendors (id, name, description, icon, status, created_time, updated_time, deleted_at) VALUES (3, 'DeepSeek', '', 'DeepSeek.Color', 1, 1772250826, 1772250826, NULL);
INSERT INTO vendors (id, name, description, icon, status, created_time, updated_time, deleted_at) VALUES (4, 'Anthropic', '', 'Claude.Color', 1, 1772256666, 1772256666, NULL);
INSERT INTO vendors (id, name, description, icon, status, created_time, updated_time, deleted_at) VALUES (5, 'OpenAI', '', 'OpenAI', 1, 1772508979, 1772508979, NULL);
INSERT INTO vendors (id, name, description, icon, status, created_time, updated_time, deleted_at) VALUES (6, 'Google', '', 'Gemini.Color', 1, 1772509916, 1772509916, NULL);
INSERT INTO vendors (id, name, description, icon, status, created_time, updated_time, deleted_at) VALUES (7, 'xAI', '', 'XAI', 1, 1772514209, 1772514209, NULL);
INSERT INTO vendors (id, name, description, icon, status, created_time, updated_time, deleted_at) VALUES (9, '智谱', '', 'Zhipu.Color', 1, 1772531010, 1772531010, NULL);
INSERT INTO vendors (id, name, description, icon, status, created_time, updated_time, deleted_at) VALUES (10, 'Moonshot', '', 'Moonshot', 1, 1772531833, 1772531833, NULL);
INSERT INTO vendors (id, name, description, icon, status, created_time, updated_time, deleted_at) VALUES (11, '讯飞', '', 'Spark.Color', 1, 1772781065, 1772781065, NULL);
INSERT INTO vendors (id, name, description, icon, status, created_time, updated_time, deleted_at) VALUES (12, 'Fedimoss', '', '', 1, 1775618912, 1775618912, NULL);
INSERT INTO vendors (id, name, description, icon, status, created_time, updated_time, deleted_at) VALUES (13, 'OpenRouter', '', '', 1, 1775619006, 1775619006, NULL);
INSERT INTO vendors (id, name, description, icon, status, created_time, updated_time, deleted_at) VALUES (14, '千问', '', 'Qwen', 1, 1777338555, 1777338555, NULL);
INSERT INTO vendors (id, name, description, icon, status, created_time, updated_time, deleted_at) VALUES (8, '阿里巴巴', '', 'Qwen.Color', 1, 1772521701, 1772521701, '2026-04-28 09:18:33.722196+08');
INSERT INTO vendors (id, name, description, icon, status, created_time, updated_time, deleted_at) VALUES (15, '阿里巴巴', '', 'Qwen.Color', 1, 1779932768, 1779932768, NULL);



SELECT pg_catalog.setval('paradedb._typmod_cache_id_seq', 1, false);



SELECT pg_catalog.setval('admin_sessions_id_seq', 1, true);



SELECT pg_catalog.setval('admin_users_id_seq', 1, true);



SELECT pg_catalog.setval('channels_id_seq', 90, true);


--
-- Name: checkins_id_seq; Type: SEQUENCE SET;;
--

SELECT pg_catalog.setval('checkins_id_seq', 4, true);


--
-- Name: consume_rebates_id_seq; Type: SEQUENCE SET;;
--

SELECT pg_catalog.setval('consume_rebates_id_seq', 434, true);


--
-- Name: crypto_transactions_id_seq; Type: SEQUENCE SET;;
--

SELECT pg_catalog.setval('crypto_transactions_id_seq', 1, false);


--
-- Name: custom_oauth_providers_id_seq; Type: SEQUENCE SET;;
--

SELECT pg_catalog.setval('custom_oauth_providers_id_seq', 1, false);


--
-- Name: epay_merchants_id_seq; Type: SEQUENCE SET;;
--

SELECT pg_catalog.setval('epay_merchants_id_seq', 1, true);


--
-- Name: invite_records_id_seq; Type: SEQUENCE SET;;
--

SELECT pg_catalog.setval('invite_records_id_seq', 13, true);


--
-- Name: login_audit_logs_id_seq; Type: SEQUENCE SET;;
--

SELECT pg_catalog.setval('login_audit_logs_id_seq', 1, true);


--
-- Name: logs_id_seq; Type: SEQUENCE SET;;
--

SELECT pg_catalog.setval('logs_id_seq', 256911, true);


--
-- Name: midjourneys_id_seq; Type: SEQUENCE SET;;
--

SELECT pg_catalog.setval('midjourneys_id_seq', 1, false);


--
-- Name: models_id_seq; Type: SEQUENCE SET;;
--

SELECT pg_catalog.setval('models_id_seq', 43, true);


--
-- Name: orders_id_seq; Type: SEQUENCE SET;;
--

SELECT pg_catalog.setval('orders_id_seq', 5, true);


--
-- Name: passkey_credentials_id_seq; Type: SEQUENCE SET;;
--

SELECT pg_catalog.setval('passkey_credentials_id_seq', 1, false);


--
-- Name: payment_bill_reconcile_id_seq; Type: SEQUENCE SET;;
--

SELECT pg_catalog.setval('payment_bill_reconcile_id_seq', 98, true);


--
-- Name: payment_bill_record_id_seq; Type: SEQUENCE SET;;
--

SELECT pg_catalog.setval('payment_bill_record_id_seq', 98, true);


--
-- Name: prefill_groups_id_seq; Type: SEQUENCE SET;;
--

SELECT pg_catalog.setval('prefill_groups_id_seq', 1, false);


--
-- Name: provider_configs_id_seq; Type: SEQUENCE SET;;
--

SELECT pg_catalog.setval('provider_configs_id_seq', 7, true);


--
-- Name: provider_domains_id_seq; Type: SEQUENCE SET;;
--

SELECT pg_catalog.setval('provider_domains_id_seq', 13, true);


--
-- Name: provider_model_pricings_id_seq; Type: SEQUENCE SET;;
--

SELECT pg_catalog.setval('provider_model_pricings_id_seq', 112, true);


--
-- Name: provider_options_id_seq; Type: SEQUENCE SET;;
--

SELECT pg_catalog.setval('provider_options_id_seq', 4, true);


--
-- Name: provider_profits_id_seq; Type: SEQUENCE SET;;
--

SELECT pg_catalog.setval('provider_profits_id_seq', 1265, true);


--
-- Name: provider_reward_configs_id_seq; Type: SEQUENCE SET;;
--

SELECT pg_catalog.setval('provider_reward_configs_id_seq', 3, true);


--
-- Name: provider_withdraw_id_seq; Type: SEQUENCE SET;;
--

SELECT pg_catalog.setval('provider_withdraw_id_seq', 1, true);


--
-- Name: providers_id_seq; Type: SEQUENCE SET;;
--

SELECT pg_catalog.setval('providers_id_seq', 8, true);


--
-- Name: quota_data_id_seq; Type: SEQUENCE SET;;
--

SELECT pg_catalog.setval('quota_data_id_seq', 4867, true);


--
-- Name: redemptions_id_seq; Type: SEQUENCE SET;;
--

SELECT pg_catalog.setval('redemptions_id_seq', 55, true);


--
-- Name: reward_records_id_seq; Type: SEQUENCE SET;;
--

SELECT pg_catalog.setval('reward_records_id_seq', 460, true);


--
-- Name: service_configs_id_seq; Type: SEQUENCE SET;;
--

SELECT pg_catalog.setval('service_configs_id_seq', 1, false);


--
-- Name: setups_id_seq; Type: SEQUENCE SET;;
--

SELECT pg_catalog.setval('setups_id_seq', 1, true);


--
-- Name: subscription_orders_id_seq; Type: SEQUENCE SET;;
--

SELECT pg_catalog.setval('subscription_orders_id_seq', 2, true);


--
-- Name: subscription_plans_id_seq; Type: SEQUENCE SET;;
--

SELECT pg_catalog.setval('subscription_plans_id_seq', 2, true);


--
-- Name: subscription_pre_consume_records_id_seq; Type: SEQUENCE SET;;
--

SELECT pg_catalog.setval('subscription_pre_consume_records_id_seq', 1, true);


--
-- Name: tasks_id_seq; Type: SEQUENCE SET;;
--

SELECT pg_catalog.setval('tasks_id_seq', 1, false);


--
-- Name: tokens_id_seq; Type: SEQUENCE SET;;
--

SELECT pg_catalog.setval('tokens_id_seq', 149, true);


--
-- Name: top_ups_id_seq; Type: SEQUENCE SET;;
--

SELECT pg_catalog.setval('top_ups_id_seq', 908, true);


--
-- Name: topup_rebates_id_seq; Type: SEQUENCE SET;;
--

SELECT pg_catalog.setval('topup_rebates_id_seq', 1, true);


--
-- Name: two_fa_backup_codes_id_seq; Type: SEQUENCE SET;;
--

SELECT pg_catalog.setval('two_fa_backup_codes_id_seq', 4, true);


--
-- Name: two_fas_id_seq; Type: SEQUENCE SET;;
--

SELECT pg_catalog.setval('two_fas_id_seq', 1, true);


--
-- Name: user_oauth_bindings_id_seq; Type: SEQUENCE SET;;
--

SELECT pg_catalog.setval('user_oauth_bindings_id_seq', 1, false);


--
-- Name: user_subscriptions_id_seq; Type: SEQUENCE SET;;
--

SELECT pg_catalog.setval('user_subscriptions_id_seq', 1, true);


--
-- Name: users_id_seq; Type: SEQUENCE SET;;
--

SELECT pg_catalog.setval('users_id_seq', 5994, true);


--
-- Name: vendors_id_seq; Type: SEQUENCE SET;;
--

SELECT pg_catalog.setval('vendors_id_seq', 15, true);


--
-- Name: version_log_id_seq; Type: SEQUENCE SET;;
--

SELECT pg_catalog.setval('version_log_id_seq', 4, true);


--
-- Name: topology_id_seq; Type: SEQUENCE SET; Schema: topology;
--

SELECT pg_catalog.setval('topology.topology_id_seq', 1, false);


--
-- Name: abilities abilities_pkey; Type: CONSTRAINT;;
--

ALTER TABLE ONLY abilities
    ADD CONSTRAINT abilities_pkey PRIMARY KEY ("group", model, channel_id);


--
-- Name: admin_sessions admin_sessions_pkey; Type: CONSTRAINT;;
--

ALTER TABLE ONLY admin_sessions
    ADD CONSTRAINT admin_sessions_pkey PRIMARY KEY (id);


--
-- Name: admin_users admin_users_pkey; Type: CONSTRAINT;;
--

ALTER TABLE ONLY admin_users
    ADD CONSTRAINT admin_users_pkey PRIMARY KEY (id);


--
-- Name: channels channels_pkey; Type: CONSTRAINT;;
--

ALTER TABLE ONLY channels
    ADD CONSTRAINT channels_pkey PRIMARY KEY (id);


--
-- Name: checkins checkins_pkey; Type: CONSTRAINT;;
--

ALTER TABLE ONLY checkins
    ADD CONSTRAINT checkins_pkey PRIMARY KEY (id);


--
-- Name: cli_oauth cli_oauth_pkey; Type: CONSTRAINT;;
--

ALTER TABLE ONLY cli_oauth
    ADD CONSTRAINT cli_oauth_pkey PRIMARY KEY (id);


--
-- Name: cli_user_oauth cli_user_oauth_pkey; Type: CONSTRAINT;;
--

ALTER TABLE ONLY cli_user_oauth
    ADD CONSTRAINT cli_user_oauth_pkey PRIMARY KEY (id);


--
-- Name: cli_user cli_user_pkey; Type: CONSTRAINT;;
--

ALTER TABLE ONLY cli_user
    ADD CONSTRAINT cli_user_pkey PRIMARY KEY (id);


--
-- Name: consume_rebates consume_rebates_pkey; Type: CONSTRAINT;;
--

ALTER TABLE ONLY consume_rebates
    ADD CONSTRAINT consume_rebates_pkey PRIMARY KEY (id);


--
-- Name: crypto_chain_config crypto_chain_config_pkey; Type: CONSTRAINT;;
--

ALTER TABLE ONLY crypto_chain_config
    ADD CONSTRAINT crypto_chain_config_pkey PRIMARY KEY (network, token_symbol);


--
-- Name: currency_stripe_config currency_stripe_config_pkey; Type: CONSTRAINT;;
--

ALTER TABLE ONLY currency_stripe_config
    ADD CONSTRAINT currency_stripe_config_pkey PRIMARY KEY (currency);


--
-- Name: custom_oauth_providers custom_oauth_providers_pkey; Type: CONSTRAINT;;
--

ALTER TABLE ONLY custom_oauth_providers
    ADD CONSTRAINT custom_oauth_providers_pkey PRIMARY KEY (id);


--
-- Name: epay_merchants epay_merchants_pkey; Type: CONSTRAINT;;
--

ALTER TABLE ONLY epay_merchants
    ADD CONSTRAINT epay_merchants_pkey PRIMARY KEY (id);


--
-- Name: cli_user idx_cli_user_user_id; Type: CONSTRAINT;;
--

ALTER TABLE ONLY cli_user
    ADD CONSTRAINT idx_cli_user_user_id UNIQUE (user_id);


--
-- Name: crypto_transactions idx_crypto_transactions_trade_no; Type: CONSTRAINT;;
--

ALTER TABLE ONLY crypto_transactions
    ADD CONSTRAINT idx_crypto_transactions_trade_no UNIQUE (trade_no);


--
-- Name: crypto_transactions idx_crypto_transactions_tx_hash; Type: CONSTRAINT;;
--

ALTER TABLE ONLY crypto_transactions
    ADD CONSTRAINT idx_crypto_transactions_tx_hash UNIQUE (tx_hash);


--
-- Name: prefill_groups idx_prefill_groups_name; Type: CONSTRAINT;;
--

ALTER TABLE ONLY prefill_groups
    ADD CONSTRAINT idx_prefill_groups_name UNIQUE (name);


--
-- Name: provider_reward_configs idx_provider_reward_configs_provider_id; Type: CONSTRAINT;;
--

ALTER TABLE ONLY provider_reward_configs
    ADD CONSTRAINT idx_provider_reward_configs_provider_id UNIQUE (provider_id);


--
-- Name: redemptions idx_redemptions_key; Type: CONSTRAINT;;
--

ALTER TABLE ONLY redemptions
    ADD CONSTRAINT idx_redemptions_key UNIQUE (key);


--
-- Name: invite_records invite_records_invitee_id_key; Type: CONSTRAINT;;
--

ALTER TABLE ONLY invite_records
    ADD CONSTRAINT invite_records_invitee_id_key UNIQUE (invitee_id);


--
-- Name: invite_records invite_records_pkey; Type: CONSTRAINT;;
--

ALTER TABLE ONLY invite_records
    ADD CONSTRAINT invite_records_pkey PRIMARY KEY (id);


--
-- Name: login_audit_logs login_audit_logs_pkey; Type: CONSTRAINT;;
--

ALTER TABLE ONLY login_audit_logs
    ADD CONSTRAINT login_audit_logs_pkey PRIMARY KEY (id);


--
-- Name: logs logs_pkey; Type: CONSTRAINT;;
--

ALTER TABLE ONLY logs
    ADD CONSTRAINT logs_pkey PRIMARY KEY (id);


--
-- Name: midjourneys midjourneys_pkey; Type: CONSTRAINT;;
--

ALTER TABLE ONLY midjourneys
    ADD CONSTRAINT midjourneys_pkey PRIMARY KEY (id);


--
-- Name: models models_pkey; Type: CONSTRAINT;;
--

ALTER TABLE ONLY models
    ADD CONSTRAINT models_pkey PRIMARY KEY (id);


--
-- Name: options options_pkey; Type: CONSTRAINT;;
--

ALTER TABLE ONLY options
    ADD CONSTRAINT options_pkey PRIMARY KEY (key);


--
-- Name: orders orders_pkey; Type: CONSTRAINT;;
--

ALTER TABLE ONLY orders
    ADD CONSTRAINT orders_pkey PRIMARY KEY (id);


--
-- Name: passkey_credentials passkey_credentials_pkey; Type: CONSTRAINT;;
--

ALTER TABLE ONLY passkey_credentials
    ADD CONSTRAINT passkey_credentials_pkey PRIMARY KEY (id);


--
-- Name: payment_bill_reconcile payment_bill_reconcile_pkey; Type: CONSTRAINT;;
--

ALTER TABLE ONLY payment_bill_reconcile
    ADD CONSTRAINT payment_bill_reconcile_pkey PRIMARY KEY (id);


--
-- Name: payment_bill_record payment_bill_record_pkey; Type: CONSTRAINT;;
--

ALTER TABLE ONLY payment_bill_record
    ADD CONSTRAINT payment_bill_record_pkey PRIMARY KEY (id);


--
-- Name: prefill_groups prefill_groups_pkey; Type: CONSTRAINT;;
--

ALTER TABLE ONLY prefill_groups
    ADD CONSTRAINT prefill_groups_pkey PRIMARY KEY (id);


--
-- Name: provider_configs provider_configs_pkey; Type: CONSTRAINT;;
--

ALTER TABLE ONLY provider_configs
    ADD CONSTRAINT provider_configs_pkey PRIMARY KEY (id);


--
-- Name: provider_domains provider_domains_pkey; Type: CONSTRAINT;;
--

ALTER TABLE ONLY provider_domains
    ADD CONSTRAINT provider_domains_pkey PRIMARY KEY (id);


--
-- Name: provider_model_pricings provider_model_pricings_pkey; Type: CONSTRAINT;;
--

ALTER TABLE ONLY provider_model_pricings
    ADD CONSTRAINT provider_model_pricings_pkey PRIMARY KEY (id);


--
-- Name: provider_profits provider_profits_pkey; Type: CONSTRAINT;;
--

ALTER TABLE ONLY provider_profits
    ADD CONSTRAINT provider_profits_pkey PRIMARY KEY (id);


--
-- Name: provider_profits provider_profits_request_id_key; Type: CONSTRAINT;;
--

ALTER TABLE ONLY provider_profits
    ADD CONSTRAINT provider_profits_request_id_key UNIQUE (request_id);


--
-- Name: provider_reward_configs provider_reward_configs_pkey; Type: CONSTRAINT;;
--

ALTER TABLE ONLY provider_reward_configs
    ADD CONSTRAINT provider_reward_configs_pkey PRIMARY KEY (id);


--
-- Name: providers providers_pkey; Type: CONSTRAINT;;
--

ALTER TABLE ONLY providers
    ADD CONSTRAINT providers_pkey PRIMARY KEY (id);


--
-- Name: quota_data quota_data_pkey; Type: CONSTRAINT;;
--

ALTER TABLE ONLY quota_data
    ADD CONSTRAINT quota_data_pkey PRIMARY KEY (id);


--
-- Name: redemptions redemptions_pkey; Type: CONSTRAINT;;
--

ALTER TABLE ONLY redemptions
    ADD CONSTRAINT redemptions_pkey PRIMARY KEY (id);


--
-- Name: reward_records reward_records_pkey; Type: CONSTRAINT;;
--

ALTER TABLE ONLY reward_records
    ADD CONSTRAINT reward_records_pkey PRIMARY KEY (id);


--
-- Name: service_configs service_configs_pkey; Type: CONSTRAINT;;
--

ALTER TABLE ONLY service_configs
    ADD CONSTRAINT service_configs_pkey PRIMARY KEY (id);


--
-- Name: setups setups_pkey; Type: CONSTRAINT;;
--

ALTER TABLE ONLY setups
    ADD CONSTRAINT setups_pkey PRIMARY KEY (id);


--
-- Name: subscription_orders subscription_orders_pkey; Type: CONSTRAINT;;
--

ALTER TABLE ONLY subscription_o
    ADD CONSTRAINT subscription_orders_pkey PRIMARY KEY (id);


--
-- Name: subscription_orders subscription_orders_trade_no_key; Type: CONSTRAINT;;
--

ALTER TABLE ONLY subscription_orders
    ADD CONSTRAINT subscription_orders_trade_no_key UNIQUE (trade_no);


--
-- Name: subscription_plans subscription_plans_pkey; Type: CONSTRAINT;;
--

ALTER TABLE ONLY subscription_plans
    ADD CONSTRAINT subscription_plans_pkey PRIMARY KEY (id);


--
-- Name: subscription_plans chk_subscription_plans_provider_id_non_negative; Type: CONSTRAINT;;
--

ALTER TABLE subscription_plans
    DROP CONSTRAINT IF EXISTS chk_subscription_plans_provider_id_non_negative;

ALTER TABLE subscription_plans
    ADD CONSTRAINT chk_subscription_plans_provider_id_non_negative
    CHECK (provider_id >= 0);


--
-- Name: subscription_pre_consume_records subscription_pre_consume_records_pkey; Type: CONSTRAINT;;
--

ALTER TABLE ONLY subscription_pre_consume_records
    ADD CONSTRAINT subscription_pre_consume_records_pkey PRIMARY KEY (id);


--
-- Name: tasks tasks_pkey; Type: CONSTRAINT;;
--

ALTER TABLE ONLY tasks
    ADD CONSTRAINT tasks_pkey PRIMARY KEY (id);


--
-- Name: timezone_currency_map timezone_currency_map_pkey; Type: CONSTRAINT;;
--

ALTER TABLE ONLY timezone_currency_map
    ADD CONSTRAINT timezone_currency_map_pkey PRIMARY KEY (timezone);


--
-- Name: tokens tokens_pkey; Type: CONSTRAINT;;
--

ALTER TABLE ONLY tokens
    ADD CONSTRAINT tokens_pkey PRIMARY KEY (id);


--
-- Name: top_ups top_ups_pkey; Type: CONSTRAINT;;
--

ALTER TABLE ONLY top_ups
    ADD CONSTRAINT top_ups_pkey PRIMARY KEY (id);


--
-- Name: top_ups top_ups_trade_no_key; Type: CONSTRAINT;;
--

ALTER TABLE ONLY top_ups
    ADD CONSTRAINT top_ups_trade_no_key UNIQUE (trade_no);


--
-- Name: topup_rebates topup_rebates_pkey; Type: CONSTRAINT;;
--

ALTER TABLE ONLY topup_rebates
    ADD CONSTRAINT topup_rebates_pkey PRIMARY KEY (id);


--
-- Name: two_fa_backup_codes two_fa_backup_codes_pkey; Type: CONSTRAINT;;
--

ALTER TABLE ONLY two_fa_backup_codes
    ADD CONSTRAINT two_fa_backup_codes_pkey PRIMARY KEY (id);


--
-- Name: two_fas two_fas_pkey; Type: CONSTRAINT;;
--

ALTER TABLE ONLY two_fas
    ADD CONSTRAINT two_fas_pkey PRIMARY KEY (id);


--
-- Name: two_fas two_fas_user_id_key; Type: CONSTRAINT;;
--

ALTER TABLE ONLY two_fas
    ADD CONSTRAINT two_fas_user_id_key UNIQUE (user_id);


--
-- Name: payment_bill_reconcile uk_payment_bill_reconcile_channel_record; Type: CONSTRAINT;;
--

ALTER TABLE ONLY payment_bill_reconcile
    ADD CONSTRAINT uk_payment_bill_reconcile_channel_record UNIQUE (channel_type, bill_record_id);


--
-- Name: payment_bill_record uk_payment_bill_record_channel_row_hash; Type: CONSTRAINT;;
--

ALTER TABLE ONLY payment_bill_record
    ADD CONSTRAINT uk_payment_bill_record_channel_row_hash UNIQUE (channel_type, row_hash);


--
-- Name: user_oauth_bindings user_oauth_bindings_pkey; Type: CONSTRAINT;;
--

ALTER TABLE ONLY user_oauth_bindings
    ADD CONSTRAINT user_oauth_bindings_pkey PRIMARY KEY (id);


--
-- Name: user_subscriptions user_subscriptions_pkey; Type: CONSTRAINT;;
--

ALTER TABLE ONLY user_subscriptions
    ADD CONSTRAINT user_subscriptions_pkey PRIMARY KEY (id);


--
-- Name: users users_pkey; Type: CONSTRAINT;;
--

ALTER TABLE ONLY users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);


--
-- Name: vendors vendors_pkey; Type: CONSTRAINT;;
--

ALTER TABLE ONLY vendors
    ADD CONSTRAINT vendors_pkey PRIMARY KEY (id);


--
-- Name: idx_abilities_channel_id; Type: INDEX;;
--

CREATE INDEX idx_abilities_channel_id ON abilities USING btree (channel_id);


--
-- Name: idx_abilities_priority; Type: INDEX;;
--

CREATE INDEX idx_abilities_priority ON abilities USING btree (priority);


--
-- Name: idx_abilities_tag; Type: INDEX;;
--

CREATE INDEX idx_abilities_tag ON abilities USING btree (tag);


--
-- Name: idx_abilities_weight; Type: INDEX;;
--

CREATE INDEX idx_abilities_weight ON abilities USING btree (weight);


--
-- Name: idx_admin_sessions_expires_at; Type: INDEX;;
--

CREATE INDEX idx_admin_sessions_expires_at ON admin_sessions USING btree (expires_at);


--
-- Name: idx_admin_sessions_token; Type: INDEX;;
--

CREATE UNIQUE INDEX idx_admin_sessions_token ON admin_sessions USING btree (token);


--
-- Name: idx_admin_sessions_user_id; Type: INDEX;;
--

CREATE INDEX idx_admin_sessions_user_id ON admin_sessions USING btree (user_id);


--
-- Name: idx_admin_users_username; Type: INDEX;;
--

CREATE UNIQUE INDEX idx_admin_users_username ON admin_users USING btree (username);


--
-- Name: idx_channels_name; Type: INDEX;;
--

CREATE INDEX idx_channels_name ON channels USING btree (name);


--
-- Name: idx_channels_tag; Type: INDEX;;
--

CREATE INDEX idx_channels_tag ON channels USING btree (tag);


--
-- Name: idx_checkins_provider_id; Type: INDEX;;
--

CREATE INDEX idx_checkins_provider_id ON checkins USING btree (provider_id);


--
-- Name: idx_consume_rebate_provider_model; Type: INDEX;;
--

CREATE INDEX idx_consume_rebate_provider_model ON consume_rebates USING btree (public_model_name);


--
-- Name: idx_consume_rebate_request_level; Type: INDEX;;
--

CREATE UNIQUE INDEX idx_consume_rebate_request_level ON consume_rebates USING btree (request_id, level);


--
-- Name: idx_consume_rebates_created_at; Type: INDEX;;
--

CREATE INDEX idx_consume_rebates_created_at ON consume_rebates USING btree (created_at);


--
-- Name: idx_consume_rebates_invitee_id; Type: INDEX;;
--

CREATE INDEX idx_consume_rebates_invitee_id ON consume_rebates USING btree (invitee_id);


--
-- Name: idx_consume_rebates_inviter_id; Type: INDEX;;
--

CREATE INDEX idx_consume_rebates_inviter_id ON consume_rebates USING btree (inviter_id);


--
-- Name: idx_consume_rebates_provider_id; Type: INDEX;;
--

CREATE INDEX idx_consume_rebates_provider_id ON consume_rebates USING btree (provider_id);


--
-- Name: idx_consume_rebates_provider_model; Type: INDEX;;
--

CREATE INDEX idx_consume_rebates_provider_model ON consume_rebates USING btree (provider_id, public_model_name);


--
-- Name: idx_consume_rebates_provider_pricing_id; Type: INDEX;;
--

CREATE INDEX idx_consume_rebates_provider_pricing_id ON consume_rebates USING btree (provider_pricing_id);


--
-- Name: idx_created_at_id; Type: INDEX;;
--

CREATE INDEX idx_created_at_id ON logs USING btree (created_at, id);


--
-- Name: idx_created_at_type; Type: INDEX;;
--

CREATE INDEX idx_created_at_type ON logs USING btree (created_at, type);


--
-- Name: idx_crypto_transactions_chain_id; Type: INDEX;;
--

CREATE INDEX idx_crypto_transactions_chain_id ON crypto_transactions USING btree (chain_id);


--
-- Name: idx_crypto_transactions_status; Type: INDEX;;
--

CREATE INDEX idx_crypto_transactions_status ON crypto_transactions USING btree (status);


--
-- Name: idx_crypto_transactions_subscription_order_id; Type: INDEX;;
--

CREATE INDEX idx_crypto_transactions_subscription_order_id ON crypto_transactions USING btree (subscription_order_id);


--
-- Name: idx_crypto_transactions_top_up_id; Type: INDEX;;
--

CREATE INDEX idx_crypto_transactions_top_up_id ON crypto_transactions USING btree (top_up_id);


--
-- Name: idx_crypto_transactions_user_id; Type: INDEX;;
--

CREATE INDEX idx_crypto_transactions_user_id ON crypto_transactions USING btree (user_id);


--
-- Name: idx_custom_oauth_providers_slug; Type: INDEX;;
--

CREATE UNIQUE INDEX idx_custom_oauth_providers_slug ON custom_oauth_providers USING btree (slug);


--
-- Name: idx_epay_merchants_p_id; Type: INDEX;;
--

CREATE UNIQUE INDEX idx_epay_merchants_p_id ON epay_merchants USING btree (pid);


--
-- Name: idx_epay_merchants_pid; Type: INDEX;;
--

CREATE UNIQUE INDEX idx_epay_merchants_pid ON epay_merchants USING btree (pid);


--
-- Name: idx_invite_records_created_at; Type: INDEX;;
--

CREATE INDEX idx_invite_records_created_at ON invite_records USING btree (created_at);


--
-- Name: idx_invite_records_invitee_id; Type: INDEX;;
--

CREATE UNIQUE INDEX idx_invite_records_invitee_id ON invite_records USING btree (invitee_id);


--
-- Name: idx_invite_records_inviter_id; Type: INDEX;;
--

CREATE INDEX idx_invite_records_inviter_id ON invite_records USING btree (inviter_id);


--
-- Name: idx_invite_records_provider_id; Type: INDEX;;
--

CREATE INDEX idx_invite_records_provider_id ON invite_records USING btree (provider_id);


--
-- Name: idx_invite_records_register_time; Type: INDEX;;
--

CREATE INDEX idx_invite_records_register_time ON invite_records USING btree (register_time);


--
-- Name: idx_login_audit_logs_created_at; Type: INDEX;;
--

CREATE INDEX idx_login_audit_logs_created_at ON login_audit_logs USING btree (created_at);


--
-- Name: idx_login_audit_logs_username; Type: INDEX;;
--

CREATE INDEX idx_login_audit_logs_username ON login_audit_logs USING btree (username);


--
-- Name: idx_logs_base_model_name; Type: INDEX;;
--

CREATE INDEX idx_logs_base_model_name ON logs USING btree (base_model_name);


--
-- Name: idx_logs_billing_side; Type: INDEX;;
--

CREATE INDEX idx_logs_billing_side ON logs USING btree (billing_side);


--
-- Name: idx_logs_channel_id; Type: INDEX;;
--

CREATE INDEX idx_logs_channel_id ON logs USING btree (channel_id);


--
-- Name: idx_logs_group; Type: INDEX;;
--

CREATE INDEX idx_logs_group ON logs USING btree ("group");


--
-- Name: idx_logs_ip; Type: INDEX;;
--

CREATE INDEX idx_logs_ip ON logs USING btree (ip);


--
-- Name: idx_logs_model_name; Type: INDEX;;
--

CREATE INDEX idx_logs_model_name ON logs USING btree (model_name);


--
-- Name: idx_logs_provider_id; Type: INDEX;;
--

CREATE INDEX idx_logs_provider_id ON logs USING btree (provider_id);


--
-- Name: idx_logs_request_id; Type: INDEX;;
--

CREATE INDEX idx_logs_request_id ON logs USING btree (request_id);


--
-- Name: idx_logs_token_id; Type: INDEX;;
--

CREATE INDEX idx_logs_token_id ON logs USING btree (token_id);


--
-- Name: idx_logs_token_name; Type: INDEX;;
--

CREATE INDEX idx_logs_token_name ON logs USING btree (token_name);


--
-- Name: idx_logs_upstream_request_id; Type: INDEX;;
--

CREATE INDEX idx_logs_upstream_request_id ON logs USING btree (upstream_request_id);


--
-- Name: idx_logs_user_id; Type: INDEX;;
--

CREATE INDEX idx_logs_user_id ON logs USING btree (user_id);


--
-- Name: idx_logs_username; Type: INDEX;;
--

CREATE INDEX idx_logs_username ON logs USING btree (username);


--
-- Name: idx_midjourneys_action; Type: INDEX;;
--

CREATE INDEX idx_midjourneys_action ON midjourneys USING btree (action);


--
-- Name: idx_midjourneys_finish_time; Type: INDEX;;
--

CREATE INDEX idx_midjourneys_finish_time ON midjourneys USING btree (finish_time);


--
-- Name: idx_midjourneys_mj_id; Type: INDEX;;
--

CREATE INDEX idx_midjourneys_mj_id ON midjourneys USING btree (mj_id);


--
-- Name: idx_midjourneys_progress; Type: INDEX;;
--

CREATE INDEX idx_midjourneys_progress ON midjourneys USING btree (progress);


--
-- Name: idx_midjourneys_start_time; Type: INDEX;;
--

CREATE INDEX idx_midjourneys_start_time ON midjourneys USING btree (start_time);


--
-- Name: idx_midjourneys_status; Type: INDEX;;
--

CREATE INDEX idx_midjourneys_status ON midjourneys USING btree (status);


--
-- Name: idx_midjourneys_submit_time; Type: INDEX;;
--

CREATE INDEX idx_midjourneys_submit_time ON midjourneys USING btree (submit_time);


--
-- Name: idx_midjourneys_user_id; Type: INDEX;;
--

CREATE INDEX idx_midjourneys_user_id ON midjourneys USING btree (user_id);


--
-- Name: idx_models_deleted_at; Type: INDEX;;
--

CREATE INDEX idx_models_deleted_at ON models USING btree (deleted_at);


--
-- Name: idx_models_vendor_id; Type: INDEX;;
--

CREATE INDEX idx_models_vendor_id ON models USING btree (vendor_id);


--
-- Name: idx_orders_callback_status; Type: INDEX;;
--

CREATE INDEX idx_orders_callback_status ON orders USING btree (callback_status);


--
-- Name: idx_orders_out_trade_no; Type: INDEX;;
--

CREATE UNIQUE INDEX idx_orders_out_trade_no ON orders USING btree (out_trade_no);


--
-- Name: idx_orders_status; Type: INDEX;;
--

CREATE INDEX idx_orders_status ON orders USING btree (status);


--
-- Name: idx_passkey_credentials_credential_id; Type: INDEX;;
--

CREATE UNIQUE INDEX idx_passkey_credentials_credential_id ON passkey_credentials USING btree (credential_id);


--
-- Name: idx_passkey_credentials_deleted_at; Type: INDEX;;
--

CREATE INDEX idx_passkey_credentials_deleted_at ON passkey_credentials USING btree (deleted_at);


--
-- Name: idx_passkey_credentials_user_id; Type: INDEX;;
--

CREATE UNIQUE INDEX idx_passkey_credentials_user_id ON passkey_credentials USING btree (user_id);


--
-- Name: idx_payment_bill_reconcile_bill_date; Type: INDEX;;
--

CREATE INDEX idx_payment_bill_reconcile_bill_date ON payment_bill_reconcile USING btree (bill_date);


--
-- Name: idx_payment_bill_reconcile_bill_record_id; Type: INDEX;;
--

CREATE INDEX idx_payment_bill_reconcile_bill_record_id ON payment_bill_reconcile USING btree (bill_record_id);


--
-- Name: idx_payment_bill_reconcile_channel_key; Type: INDEX;;
--

CREATE UNIQUE INDEX idx_payment_bill_reconcile_channel_key ON payment_bill_reconcile USING btree (channel_type, reconcile_key);


--
-- Name: idx_payment_bill_reconcile_channel_status; Type: INDEX;;
--

CREATE INDEX idx_payment_bill_reconcile_channel_status ON payment_bill_reconcile USING btree (channel_status);


--
-- Name: idx_payment_bill_reconcile_channel_trade_no; Type: INDEX;;
--

CREATE INDEX idx_payment_bill_reconcile_channel_trade_no ON payment_bill_reconcile USING btree (channel_trade_no);


--
-- Name: idx_payment_bill_reconcile_channel_type; Type: INDEX;;
--

CREATE INDEX idx_payment_bill_reconcile_channel_type ON payment_bill_reconcile USING btree (channel_type);


--
-- Name: idx_payment_bill_reconcile_local_complete_time; Type: INDEX;;
--

CREATE INDEX idx_payment_bill_reconcile_local_complete_time ON payment_bill_reconcile USING btree (local_complete_time);


--
-- Name: idx_payment_bill_reconcile_local_create_time; Type: INDEX;;
--

CREATE INDEX idx_payment_bill_reconcile_local_create_time ON payment_bill_reconcile USING btree (local_create_time);


--
-- Name: idx_payment_bill_reconcile_local_id; Type: INDEX;;
--

CREATE INDEX idx_payment_bill_reconcile_local_id ON payment_bill_reconcile USING btree (local_id);


--
-- Name: idx_payment_bill_reconcile_local_status; Type: INDEX;;
--

CREATE INDEX idx_payment_bill_reconcile_local_status ON payment_bill_reconcile USING btree (local_status);


--
-- Name: idx_payment_bill_reconcile_local_trade_no; Type: INDEX;;
--

CREATE INDEX idx_payment_bill_reconcile_local_trade_no ON payment_bill_reconcile USING btree (local_trade_no);


--
-- Name: idx_payment_bill_reconcile_local_type; Type: INDEX;;
--

CREATE INDEX idx_payment_bill_reconcile_local_type ON payment_bill_reconcile USING btree (local_type);


--
-- Name: idx_payment_bill_reconcile_merchant_trade_no; Type: INDEX;;
--

CREATE INDEX idx_payment_bill_reconcile_merchant_trade_no ON payment_bill_reconcile USING btree (merchant_trade_no);


--
-- Name: idx_payment_bill_reconcile_reconcile_reason; Type: INDEX;;
--

CREATE INDEX idx_payment_bill_reconcile_reconcile_reason ON payment_bill_reconcile USING btree (reconcile_reason);


--
-- Name: idx_payment_bill_reconcile_reconcile_status; Type: INDEX;;
--

CREATE INDEX idx_payment_bill_reconcile_reconcile_status ON payment_bill_reconcile USING btree (reconcile_status);


--
-- Name: idx_payment_bill_reconcile_record_source; Type: INDEX;;
--

CREATE INDEX idx_payment_bill_reconcile_record_source ON payment_bill_reconcile USING btree (record_source);


--
-- Name: idx_payment_bill_reconcile_trade_time; Type: INDEX;;
--

CREATE INDEX idx_payment_bill_reconcile_trade_time ON payment_bill_reconcile USING btree (trade_time);


--
-- Name: idx_payment_bill_record_bill_date; Type: INDEX;;
--

CREATE INDEX idx_payment_bill_record_bill_date ON payment_bill_record USING btree (bill_date);


--
-- Name: idx_payment_bill_record_channel_refund_no; Type: INDEX;;
--

CREATE INDEX idx_payment_bill_record_channel_refund_no ON payment_bill_record USING btree (channel_refund_no);


--
-- Name: idx_payment_bill_record_channel_row_hash; Type: INDEX;;
--

CREATE UNIQUE INDEX idx_payment_bill_record_channel_row_hash ON payment_bill_record USING btree (channel_type, row_hash);


--
-- Name: idx_payment_bill_record_channel_trade_no; Type: INDEX;;
--

CREATE INDEX idx_payment_bill_record_channel_trade_no ON payment_bill_record USING btree (channel_trade_no);


--
-- Name: idx_payment_bill_record_channel_type; Type: INDEX;;
--

CREATE INDEX idx_payment_bill_record_channel_type ON payment_bill_record USING btree (channel_type);


--
-- Name: idx_payment_bill_record_mch_id; Type: INDEX;;
--

CREATE INDEX idx_payment_bill_record_mch_id ON payment_bill_record USING btree (mch_id);


--
-- Name: idx_payment_bill_record_merchant_refund_no; Type: INDEX;;
--

CREATE INDEX idx_payment_bill_record_merchant_refund_no ON payment_bill_record USING btree (merchant_refund_no);


--
-- Name: idx_payment_bill_record_merchant_trade_no; Type: INDEX;;
--

CREATE INDEX idx_payment_bill_record_merchant_trade_no ON payment_bill_record USING btree (merchant_trade_no);


--
-- Name: idx_payment_bill_record_row_index; Type: INDEX;;
--

CREATE INDEX idx_payment_bill_record_row_index ON payment_bill_record USING btree (row_index);


--
-- Name: idx_payment_bill_record_trade_status; Type: INDEX;;
--

CREATE INDEX idx_payment_bill_record_trade_status ON payment_bill_record USING btree (trade_status);


--
-- Name: idx_payment_bill_record_trade_time; Type: INDEX;;
--

CREATE INDEX idx_payment_bill_record_trade_time ON payment_bill_record USING btree (trade_time);


--
-- Name: idx_prefill_groups_deleted_at; Type: INDEX;;
--

CREATE INDEX idx_prefill_groups_deleted_at ON prefill_groups USING btree (deleted_at);


--
-- Name: idx_prefill_groups_type; Type: INDEX;;
--

CREATE INDEX idx_prefill_groups_type ON prefill_groups USING btree (type);


--
-- Name: idx_provider_configs_model_pricing_sync_enabled; Type: INDEX;;
--

CREATE INDEX idx_provider_configs_model_pricing_sync_enabled ON provider_configs USING btree (model_pricing_sync_enabled);


--
-- Name: idx_provider_configs_provider_id; Type: INDEX;;
--

CREATE UNIQUE INDEX idx_provider_configs_provider_id ON provider_configs USING btree (provider_id);


--
-- Name: idx_provider_domains_domain; Type: INDEX;;
--

CREATE UNIQUE INDEX idx_provider_domains_domain ON provider_domains USING btree (domain);


--
-- Name: idx_provider_domains_provider_id; Type: INDEX;;
--

CREATE INDEX idx_provider_domains_provider_id ON provider_domains USING btree (provider_id);


--
-- Name: idx_provider_domains_status; Type: INDEX;;
--

CREATE INDEX idx_provider_domains_status ON provider_domains USING btree (status);


--
-- Name: idx_provider_model_pricings_base_model_name; Type: INDEX;;
--

CREATE INDEX idx_provider_model_pricings_base_model_name ON provider_model_pricings USING btree (base_model_name);


--
-- Name: idx_provider_model_pricings_enabled; Type: INDEX;;
--

CREATE INDEX idx_provider_model_pricings_enabled ON provider_model_pricings USING btree (enabled);


--
-- Name: idx_provider_model_pricings_provider_id; Type: INDEX;;
--

CREATE INDEX idx_provider_model_pricings_provider_id ON provider_model_pricings USING btree (provider_id);


--
-- Name: idx_provider_model_pricings_sync_disabled; Type: INDEX;;
--

CREATE INDEX idx_provider_model_pricings_sync_disabled ON provider_model_pricings USING btree (sync_disabled);


--
-- Name: idx_provider_options_provider_id; Type: INDEX;;
--

CREATE INDEX idx_provider_options_provider_id ON provider_options USING btree (provider_id);


--
-- Name: idx_provider_profits_created_at; Type: INDEX;;
--

CREATE INDEX idx_provider_profits_created_at ON provider_profits USING btree (created_at);


--
-- Name: idx_provider_profits_owner_cost_settled; Type: INDEX;;
--

CREATE INDEX idx_provider_profits_owner_cost_settled ON provider_profits USING btree (owner_cost_settled);


--
-- Name: idx_provider_profits_owner_user_id; Type: INDEX;;
--

CREATE INDEX idx_provider_profits_owner_user_id ON provider_profits USING btree (owner_user_id);


--
-- Name: idx_provider_profits_profit_settled; Type: INDEX;;
--

CREATE INDEX idx_provider_profits_profit_settled ON provider_profits USING btree (profit_settled);


--
-- Name: idx_provider_profits_provider_id; Type: INDEX;;
--

CREATE INDEX idx_provider_profits_provider_id ON provider_profits USING btree (provider_id);


--
-- Name: idx_provider_profits_provider_user_id; Type: INDEX;;
--

CREATE INDEX idx_provider_profits_provider_user_id ON provider_profits USING btree (provider_user_id);


--
-- Name: idx_provider_profits_request_id; Type: INDEX;;
--

CREATE UNIQUE INDEX idx_provider_profits_request_id ON provider_profits USING btree (request_id);


--
-- Name: idx_provider_public_model; Type: INDEX;;
--

CREATE UNIQUE INDEX idx_provider_public_model ON provider_model_pricings USING btree (provider_id, public_model_name);


--
-- Name: idx_provider_user_checkin_date; Type: INDEX;;
--

CREATE UNIQUE INDEX idx_provider_user_checkin_date ON checkins USING btree (provider_id, user_id, checkin_date);


--
-- Name: idx_provider_withdraw_provider_id; Type: INDEX;;
--

CREATE INDEX idx_provider_withdraw_provider_id ON provider_withdraw USING btree (provider_id);


--
-- Name: idx_provider_withdraw_status; Type: INDEX;;
--

CREATE INDEX idx_provider_withdraw_status ON provider_withdraw USING btree (status);


--
-- Name: idx_providers_owner_user_id; Type: INDEX;;
--

CREATE INDEX idx_providers_owner_user_id ON providers USING btree (owner_user_id);


--
-- Name: idx_providers_status; Type: INDEX;;
--

CREATE INDEX idx_providers_status ON providers USING btree (status);


--
-- Name: idx_qdt_created_at; Type: INDEX;;
--

CREATE INDEX idx_qdt_created_at ON quota_data USING btree (created_at);


--
-- Name: idx_qdt_model_user_name; Type: INDEX;;
--

CREATE INDEX idx_qdt_model_user_name ON quota_data USING btree (model_name, username);


--
-- Name: idx_quota_data_user_id; Type: INDEX;;
--

CREATE INDEX idx_quota_data_user_id ON quota_data USING btree (user_id);


--
-- Name: idx_redemptions_deleted_at; Type: INDEX;;
--

CREATE INDEX idx_redemptions_deleted_at ON redemptions USING btree (deleted_at);


--
-- Name: idx_redemptions_name; Type: INDEX;;
--

CREATE INDEX idx_redemptions_name ON redemptions USING btree (name);


--
-- Name: idx_redemptions_provider_id; Type: INDEX;;
--

CREATE INDEX idx_redemptions_provider_id ON redemptions USING btree (provider_id);


--
-- Name: idx_reward_records_created_at; Type: INDEX;;
--

CREATE INDEX idx_reward_records_created_at ON reward_records USING btree (created_at);


--
-- Name: idx_reward_records_provider_id; Type: INDEX;;
--

CREATE INDEX idx_reward_records_provider_id ON reward_records USING btree (provider_id);


--
-- Name: idx_reward_records_source; Type: INDEX;;
--

CREATE INDEX idx_reward_records_source ON reward_records USING btree (source_type, source_id);


--
-- Name: idx_reward_records_user_id; Type: INDEX;;
--

CREATE INDEX idx_reward_records_user_id ON reward_records USING btree (user_id);


--
-- Name: idx_reward_source; Type: INDEX;;
--

CREATE INDEX idx_reward_source ON reward_records USING btree (source_type, source_id);


--
-- Name: idx_subscription_orders_plan_id; Type: INDEX;;
--

CREATE INDEX idx_subscription_orders_plan_id ON subscription_orders USING btree (plan_id);


--
-- Name: idx_subscription_orders_trade_no; Type: INDEX;;
--

CREATE INDEX idx_subscription_orders_trade_no ON subscription_orders USING btree (trade_no);


--
-- Name: idx_subscription_orders_user_id; Type: INDEX;;
--

CREATE INDEX idx_subscription_orders_user_id ON subscription_orders USING btree (user_id);


--
-- Name: idx_subscription_pre_consume_records_request_id; Type: INDEX;;
--

CREATE UNIQUE INDEX idx_subscription_pre_consume_records_request_id ON subscription_pre_consume_records USING btree (request_id);


--
-- Name: idx_subscription_pre_consume_records_status; Type: INDEX;;
--

CREATE INDEX idx_subscription_pre_consume_records_status ON subscription_pre_consume_records USING btree (status);


--
-- Name: idx_subscription_pre_consume_records_updated_at; Type: INDEX;;
--

CREATE INDEX idx_subscription_pre_consume_records_updated_at ON subscription_pre_consume_records USING btree (updated_at);


--
-- Name: idx_subscription_pre_consume_records_user_id; Type: INDEX;;
--

CREATE INDEX idx_subscription_pre_consume_records_user_id ON subscription_pre_consume_records USING btree (user_id);


--
-- Name: idx_subscription_pre_consume_records_user_subscription_id; Type: INDEX;;
--

CREATE INDEX idx_subscription_pre_consume_records_user_subscription_id ON subscription_pre_consume_records USING btree (user_subscription_id);


--
-- Name: idx_subscription_plans_provider_id; Type: INDEX;;
--

CREATE INDEX idx_subscription_plans_provider_id ON subscription_plans USING btree (provider_id);


--
-- Name: idx_subscription_plans_provider_enabled_sort; Type: INDEX;;
--

CREATE INDEX idx_subscription_plans_provider_enabled_sort ON subscription_plans USING btree (provider_id, enabled, sort_order DESC, id DESC);


--
-- Name: idx_tasks_action; Type: INDEX;;
--

CREATE INDEX idx_tasks_action ON tasks USING btree (action);


--
-- Name: idx_tasks_channel_id; Type: INDEX;;
--

CREATE INDEX idx_tasks_channel_id ON tasks USING btree (channel_id);


--
-- Name: idx_tasks_created_at; Type: INDEX;;
--

CREATE INDEX idx_tasks_created_at ON tasks USING btree (created_at);


--
-- Name: idx_tasks_finish_time; Type: INDEX;;
--

CREATE INDEX idx_tasks_finish_time ON tasks USING btree (finish_time);


--
-- Name: idx_tasks_platform; Type: INDEX;;
--

CREATE INDEX idx_tasks_platform ON tasks USING btree (platform);


--
-- Name: idx_tasks_progress; Type: INDEX;;
--

CREATE INDEX idx_tasks_progress ON tasks USING btree (progress);


--
-- Name: idx_tasks_start_time; Type: INDEX;;
--

CREATE INDEX idx_tasks_start_time ON tasks USING btree (start_time);


--
-- Name: idx_tasks_status; Type: INDEX;;
--

CREATE INDEX idx_tasks_status ON tasks USING btree (status);


--
-- Name: idx_tasks_submit_time; Type: INDEX;;
--

CREATE INDEX idx_tasks_submit_time ON tasks USING btree (submit_time);


--
-- Name: idx_tasks_task_id; Type: INDEX;;
--

CREATE INDEX idx_tasks_task_id ON tasks USING btree (task_id);


--
-- Name: idx_tasks_user_id; Type: INDEX;;
--

CREATE INDEX idx_tasks_user_id ON tasks USING btree (user_id);


--
-- Name: idx_tokens_deleted_at; Type: INDEX;;
--

CREATE INDEX idx_tokens_deleted_at ON tokens USING btree (deleted_at);


--
-- Name: idx_tokens_key; Type: INDEX;;
--

CREATE UNIQUE INDEX idx_tokens_key ON tokens USING btree (key);


--
-- Name: idx_tokens_name; Type: INDEX;;
--

CREATE INDEX idx_tokens_name ON tokens USING btree (name);


--
-- Name: idx_tokens_provider_id; Type: INDEX;;
--

CREATE INDEX idx_tokens_provider_id ON tokens USING btree (provider_id);


--
-- Name: idx_tokens_user_id; Type: INDEX;;
--

CREATE INDEX idx_tokens_user_id ON tokens USING btree (user_id);


--
-- Name: idx_tokens_user_provider; Type: INDEX;;
--

CREATE INDEX idx_tokens_user_provider ON tokens USING btree (user_id, provider_id);


--
-- Name: idx_top_ups_biz_type; Type: INDEX;;
--

CREATE INDEX idx_top_ups_biz_type ON top_ups USING btree (biz_type);


--
-- Name: idx_top_ups_provider_id; Type: INDEX;;
--

CREATE INDEX idx_top_ups_provider_id ON top_ups USING btree (provider_id);


--
-- Name: idx_top_ups_source_id; Type: INDEX;;
--

CREATE INDEX idx_top_ups_source_id ON top_ups USING btree (source_id);


--
-- Name: idx_top_ups_trade_no; Type: INDEX;;
--

CREATE INDEX idx_top_ups_trade_no ON top_ups USING btree (trade_no);


--
-- Name: idx_top_ups_user_id; Type: INDEX;;
--

CREATE INDEX idx_top_ups_user_id ON top_ups USING btree (user_id);


--
-- Name: idx_top_ups_user_provider; Type: INDEX;;
--

CREATE INDEX idx_top_ups_user_provider ON top_ups USING btree (user_id, provider_id);


--
-- Name: idx_top_ups_provider_subscription_source; Type: INDEX;;
--

CREATE INDEX idx_top_ups_provider_subscription_source ON top_ups USING btree (provider_id, source_id) WHERE (payment_method = 'provider_subscription');


--
-- Name: idx_topup_rebates_created_at; Type: INDEX;;
--

CREATE INDEX idx_topup_rebates_created_at ON topup_rebates USING btree (created_at);


--
-- Name: idx_topup_rebates_invitee_id; Type: INDEX;;
--

CREATE INDEX idx_topup_rebates_invitee_id ON topup_rebates USING btree (invitee_id);


--
-- Name: idx_topup_rebates_inviter_id; Type: INDEX;;
--

CREATE INDEX idx_topup_rebates_inviter_id ON topup_rebates USING btree (inviter_id);


--
-- Name: idx_topup_rebates_provider_id; Type: INDEX;;
--

CREATE INDEX idx_topup_rebates_provider_id ON topup_rebates USING btree (provider_id);


--
-- Name: idx_topup_rebates_top_up_id; Type: INDEX;;
--

CREATE UNIQUE INDEX idx_topup_rebates_top_up_id ON topup_rebates USING btree (topup_id);


--
-- Name: idx_topup_rebates_topup_id; Type: INDEX;;
--

CREATE UNIQUE INDEX idx_topup_rebates_topup_id ON topup_rebates USING btree (topup_id);


--
-- Name: idx_topup_rebates_trade_no; Type: INDEX;;
--

CREATE INDEX idx_topup_rebates_trade_no ON topup_rebates USING btree (trade_no);


--
-- Name: idx_two_fa_backup_codes_deleted_at; Type: INDEX;;
--

CREATE INDEX idx_two_fa_backup_codes_deleted_at ON two_fa_backup_codes USING btree (deleted_at);


--
-- Name: idx_two_fa_backup_codes_user_id; Type: INDEX;;
--

CREATE INDEX idx_two_fa_backup_codes_user_id ON two_fa_backup_codes USING btree (user_id);


--
-- Name: idx_two_fas_deleted_at; Type: INDEX;;
--

CREATE INDEX idx_two_fas_deleted_at ON two_fas USING btree (deleted_at);


--
-- Name: idx_two_fas_user_id; Type: INDEX;;
--

CREATE INDEX idx_two_fas_user_id ON two_fas USING btree (user_id);


--
-- Name: idx_user_checkin_date; Type: INDEX;;
--

CREATE UNIQUE INDEX idx_user_checkin_date ON checkins USING btree (user_id, checkin_date);


--
-- Name: idx_user_id; Type: INDEX;;
--

CREATE UNIQUE INDEX idx_user_id ON cli_user USING btree (user_id);


--
-- Name: idx_user_id_id; Type: INDEX;;
--

CREATE INDEX idx_user_id_id ON logs USING btree (user_id, id);


--
-- Name: idx_user_sub_active; Type: INDEX;;
--

CREATE INDEX idx_user_sub_active ON user_subscriptions USING btree (user_id, status, end_time);


--
-- Name: idx_user_subscriptions_end_time; Type: INDEX;;
--

CREATE INDEX idx_user_subscriptions_end_time ON user_subscriptions USING btree (end_time);


--
-- Name: idx_user_subscriptions_next_reset_time; Type: INDEX;;
--

CREATE INDEX idx_user_subscriptions_next_reset_time ON user_subscriptions USING btree (next_reset_time);


--
-- Name: idx_user_subscriptions_plan_id; Type: INDEX;;
--

CREATE INDEX idx_user_subscriptions_plan_id ON user_subscriptions USING btree (plan_id);


--
-- Name: idx_user_subscriptions_status; Type: INDEX;;
--

CREATE INDEX idx_user_subscriptions_status ON user_subscriptions USING btree (status);


--
-- Name: idx_user_subscriptions_user_id; Type: INDEX;;
--

CREATE INDEX idx_user_subscriptions_user_id ON user_subscriptions USING btree (user_id);


--
-- Name: idx_users_access_token; Type: INDEX;;
--

CREATE UNIQUE INDEX idx_users_access_token ON users USING btree (access_token);


--
-- Name: idx_users_aff_code; Type: INDEX;;
--

CREATE UNIQUE INDEX idx_users_aff_code ON users USING btree (aff_code);


--
-- Name: idx_users_deleted_at; Type: INDEX;;
--

CREATE INDEX idx_users_deleted_at ON users USING btree (deleted_at);


--
-- Name: idx_users_discord_id; Type: INDEX;;
--

CREATE INDEX idx_users_discord_id ON users USING btree (discord_id);


--
-- Name: idx_users_display_name; Type: INDEX;;
--

CREATE INDEX idx_users_display_name ON users USING btree (display_name);


--
-- Name: idx_users_email; Type: INDEX;;
--

CREATE INDEX idx_users_email ON users USING btree (email);


--
-- Name: idx_users_git_hub_id; Type: INDEX;;
--

CREATE INDEX idx_users_git_hub_id ON users USING btree (github_id);


--
-- Name: idx_users_inviter_id; Type: INDEX;;
--

CREATE INDEX idx_users_inviter_id ON users USING btree (inviter_id);


--
-- Name: idx_users_linux_do_id; Type: INDEX;;
--

CREATE INDEX idx_users_linux_do_id ON users USING btree (linux_do_id);


--
-- Name: idx_users_oidc_id; Type: INDEX;;
--

CREATE INDEX idx_users_oidc_id ON users USING btree (oidc_id);


--
-- Name: idx_users_provider_email; Type: INDEX;;
--

CREATE UNIQUE INDEX idx_users_provider_email ON users USING btree (provider_id, email) WHERE ((email IS NOT NULL) AND (email <> ''::text));


--
-- Name: idx_users_provider_id; Type: INDEX;;
--

CREATE INDEX idx_users_provider_id ON users USING btree (provider_id);


--
-- Name: idx_users_provider_username; Type: INDEX;;
--

CREATE UNIQUE INDEX idx_users_provider_username ON users USING btree (provider_id, username);


--
-- Name: idx_users_stripe_customer; Type: INDEX;;
--

CREATE INDEX idx_users_stripe_customer ON users USING btree (stripe_customer);


--
-- Name: idx_users_telegram_id; Type: INDEX;;
--

CREATE INDEX idx_users_telegram_id ON users USING btree (telegram_id);


--
-- Name: idx_users_username; Type: INDEX;;
--

CREATE INDEX idx_users_username ON users USING btree (username);


--
-- Name: idx_users_we_chat_id; Type: INDEX;;
--

CREATE INDEX idx_users_we_chat_id ON users USING btree (wechat_id);


--
-- Name: idx_vendors_deleted_at; Type: INDEX;;
--

CREATE INDEX idx_vendors_deleted_at ON vendors USING btree (deleted_at);


--
-- Name: index_username_model_name; Type: INDEX;;
--

CREATE INDEX index_username_model_name ON logs USING btree (model_name, username);


--
-- Name: uk_model_name_delete_at; Type: INDEX;;
--

CREATE UNIQUE INDEX uk_model_name_delete_at ON models USING btree (model_name, deleted_at);


--
-- Name: uk_prefill_name; Type: INDEX;;
--

CREATE UNIQUE INDEX uk_prefill_name ON prefill_groups USING btree (name) WHERE (deleted_at IS NULL);


--
-- Name: uk_vendor_name_delete_at; Type: INDEX;;
--

CREATE UNIQUE INDEX uk_vendor_name_delete_at ON vendors USING btree (name, deleted_at);


--
-- Name: ux_provider_redemption_key; Type: INDEX;;
--

CREATE UNIQUE INDEX ux_provider_redemption_key ON redemptions USING btree (provider_id, key);


--
-- Name: ux_provider_reward_configs_provider_id; Type: INDEX;;
--

CREATE UNIQUE INDEX ux_provider_reward_configs_provider_id ON provider_reward_configs USING btree (provider_id);


--
-- Name: ux_provider_userid; Type: INDEX;;
--

CREATE UNIQUE INDEX ux_provider_userid ON user_oauth_bindings USING btree (provider_id, provider_user_id);


--
-- Name: ux_reward_records_source_user; Type: INDEX;;
--

CREATE UNIQUE INDEX ux_reward_records_source_user ON reward_records USING btree (provider_id, source_type, source_id, user_id);


--
-- Name: ux_user_provider; Type: INDEX;;
--

CREATE UNIQUE INDEX ux_user_provider ON user_oauth_bindings USING btree (user_id, provider_id);


--
-- Name: ux_user_provider_aff; Type: INDEX;;
--

CREATE UNIQUE INDEX ux_user_provider_aff ON users USING btree (provider_id, aff_code);


--
-- Name: timezone_currency_map timezone_currency_map_currency_fkey; Type: FK CONSTRAINT;;
--

ALTER TABLE ONLY timezone_currency_map
    ADD CONSTRAINT timezone_currency_map_currency_fkey FOREIGN KEY (currency) REFERENCES currency_stripe_config(currency);


--
-- PostgreSQL database dump complete
--

\unrestrict FIqAjmpw4zcALtpViQTfH3JhEOyV1EoM9Of3mq2OLzJR4Vmvp2YCaeAsTZjY4Nq


--
-- 存储 Telegram 用户与网站用户的绑定关系
--

CREATE TABLE IF NOT EXISTS telegram_user_bindings (
    id                SERIAL       PRIMARY KEY,
    telegram_user_id  VARCHAR(64)  NOT NULL,
    user_id           INTEGER      NOT NULL,
    user_name         VARCHAR(50)  NOT NULL,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_telegram_user_id  UNIQUE (telegram_user_id),
    CONSTRAINT uq_website_user_id   UNIQUE (user_id),
    CONSTRAINT fk_website_user
        FOREIGN KEY (user_id) REFERENCES users (id)
        ON DELETE CASCADE
);

-- 表注释
COMMENT ON TABLE telegram_user_bindings IS '存储 Telegram 用户与网站用户的绑定关系';

-- 列注释
COMMENT ON COLUMN telegram_user_bindings.id                IS '自增主键，唯一标识每条绑定记录';
COMMENT ON COLUMN telegram_user_bindings.telegram_user_id IS 'Telegram 平台用户 ID，唯一且不可为空';
COMMENT ON COLUMN telegram_user_bindings.user_id           IS '关联的网站用户 ID，外键引用 users 表，唯一且不可为空';
COMMENT ON COLUMN telegram_user_bindings.user_name         IS '绑定的网站用户名（冗余存储，便于快速展示）';
COMMENT ON COLUMN telegram_user_bindings.created_at        IS '绑定记录的创建时间，默认当前时刻';
