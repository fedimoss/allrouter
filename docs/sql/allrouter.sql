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


CREATE EXTENSION IF NOT EXISTS pg_search WITH SCHEMA paradedb;



COMMENT ON EXTENSION pg_search IS 'pg_search: Full text search for PostgreSQL using BM25';



CREATE EXTENSION IF NOT EXISTS pg_ivm WITH SCHEMA pg_catalog;


COMMENT ON EXTENSION pg_ivm IS 'incremental view maintenance on PostgreSQL';



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


--
-- Name: checkins; Type: TABLE; Schema: public; Owner: allrouter
--

CREATE TABLE checkins (
    id bigint NOT NULL,
    user_id bigint NOT NULL,
    checkin_date character varying(10) NOT NULL,
    quota_awarded bigint NOT NULL,
    created_at bigint
);



CREATE SEQUENCE checkins_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;



ALTER SEQUENCE checkins_id_seq OWNED BY checkins.id;


--
-- Name: cli_oauth; Type: TABLE; Schema: public; Owner: allrouter
--

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


--
-- Name: COLUMN cli_oauth.oauth; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN cli_oauth.oauth IS 'OAuth 凭证';


--
-- Name: COLUMN cli_oauth.model_type; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN cli_oauth.model_type IS '1: Codex 2: Anthropic 3: Qwen';


--
-- Name: COLUMN cli_oauth.created_at; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN cli_oauth.created_at IS '创建时间';


--
-- Name: COLUMN cli_oauth.updated_at; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN cli_oauth.updated_at IS '更新时间';


--
-- Name: COLUMN cli_oauth.status; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN cli_oauth.status IS '状态 (1:正常 2:禁用)';


--
-- Name: COLUMN cli_oauth.account_id; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN cli_oauth.account_id IS '账户ID';


--
-- Name: cli_user; Type: TABLE; Schema: public; Owner: allrouter
--

CREATE TABLE cli_user (
    id character varying(50) NOT NULL,
    status bigint DEFAULT 1,
    user_id character varying(50),
    created_at timestamp(6) with time zone,
    updated_at timestamp(6) with time zone
);



COMMENT ON TABLE cli_user IS 'CLI 用户表';


--
-- Name: COLUMN cli_user.id; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN cli_user.id IS 'ID';


--
-- Name: COLUMN cli_user.status; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN cli_user.status IS '状态 (1:正常 2:禁用 3:删除)';


--
-- Name: COLUMN cli_user.user_id; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN cli_user.user_id IS '用户ID';


--
-- Name: COLUMN cli_user.created_at; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN cli_user.created_at IS '创建时间';


--
-- Name: COLUMN cli_user.updated_at; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN cli_user.updated_at IS '更新时间';


--
-- Name: cli_user_oauth; Type: TABLE; Schema: public; Owner: allrouter
--

CREATE TABLE cli_user_oauth (
    id character varying(50) NOT NULL,
    cli_user_id character varying(50) NOT NULL,
    cli_oauth_id character varying(50) NOT NULL
);



COMMENT ON TABLE cli_user_oauth IS 'CLI 用户凭证关联表';


--
-- Name: COLUMN cli_user_oauth.id; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN cli_user_oauth.id IS 'ID';


--
-- Name: COLUMN cli_user_oauth.cli_user_id; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN cli_user_oauth.cli_user_id IS 'CLI 用户ID';


--
-- Name: COLUMN cli_user_oauth.cli_oauth_id; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN cli_user_oauth.cli_oauth_id IS 'CLI 认证ID';


--
-- Name: currency_stripe_config; Type: TABLE; Schema: public; Owner: allrouter
--

CREATE TABLE currency_stripe_config (
    currency character varying(3) NOT NULL,
    stripe_price_id character varying(255) DEFAULT ''::character varying NOT NULL,
    unit_price numeric(18,6) DEFAULT 0 NOT NULL,
    symbol character varying(10) NOT NULL,
    updated_at timestamp with time zone DEFAULT now()
);


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


CREATE SEQUENCE custom_oauth_providers_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE custom_oauth_providers_id_seq OWNED BY custom_oauth_providers.id;


--
-- Name: epay_merchants; Type: TABLE; Schema: public; Owner: allrouter
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


--
-- Name: invite_records; Type: TABLE; Schema: public; Owner: allrouter
--

CREATE TABLE invite_records (
    id integer NOT NULL,
    inviter_id bigint,
    invitee_id bigint,
    register_time bigint NOT NULL,
    reward_quota bigint DEFAULT 0,
    created_at bigint NOT NULL
);


ALTER TABLE invite_records ALTER COLUMN id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME invite_records_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: login_audit_logs; Type: TABLE; Schema: public; Owner: allrouter
--

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


--
-- Name: logs; Type: TABLE; Schema: public; Owner: allrouter
--

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
    other text
);


CREATE SEQUENCE logs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;



ALTER SEQUENCE logs_id_seq OWNED BY logs.id;


--
-- Name: midjourneys; Type: TABLE; Schema: public; Owner: allrouter
--

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



CREATE SEQUENCE midjourneys_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE midjourneys_id_seq OWNED BY midjourneys.id;


--
-- Name: models; Type: TABLE; Schema: public; Owner: allrouter
--

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
    name_rule bigint DEFAULT 0
);



CREATE SEQUENCE models_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;



ALTER SEQUENCE models_id_seq OWNED BY models.id;


--
-- Name: options; Type: TABLE; Schema: public; Owner: allrouter
--

CREATE TABLE options (
    key text NOT NULL,
    value text
);



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



CREATE SEQUENCE orders_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;



ALTER SEQUENCE orders_id_seq OWNED BY orders.id;


--
-- Name: passkey_credentials; Type: TABLE; Schema: public; Owner: allrouter
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


--
-- Name: payment_bill_reconcile; Type: TABLE; Schema: public; Owner: allrouter
--

CREATE TABLE payment_bill_reconcile (
    id bigint NOT NULL,
    channel_type character varying(32) DEFAULT ''::character varying,
    bill_record_id bigint DEFAULT 0,
    bill_date character varying(16) DEFAULT ''::character varying,
    trade_time character varying(64) DEFAULT ''::character varying,
    channel_trade_no character varying(64) DEFAULT ''::character varying,
    merchant_trade_no character varying(64) DEFAULT ''::character varying,
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
    record_source character varying(32)
);



COMMENT ON TABLE payment_bill_reconcile IS '通用支付渠道对账结果表';


--
-- Name: COLUMN payment_bill_reconcile.id; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN payment_bill_reconcile.id IS '主键ID';


--
-- Name: COLUMN payment_bill_reconcile.channel_type; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN payment_bill_reconcile.channel_type IS '支付渠道类型，如 wxpay、stripe';


--
-- Name: COLUMN payment_bill_reconcile.bill_record_id; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN payment_bill_reconcile.bill_record_id IS '关联 payment_bill_record.id';


--
-- Name: COLUMN payment_bill_reconcile.bill_date; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN payment_bill_reconcile.bill_date IS '账单日期，格式 YYYY-MM-DD';


--
-- Name: COLUMN payment_bill_reconcile.trade_time; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN payment_bill_reconcile.trade_time IS '渠道账单中的交易时间';


--
-- Name: COLUMN payment_bill_reconcile.channel_trade_no; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN payment_bill_reconcile.channel_trade_no IS '支付渠道交易单号';


--
-- Name: COLUMN payment_bill_reconcile.merchant_trade_no; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN payment_bill_reconcile.merchant_trade_no IS '商户订单号';


--
-- Name: COLUMN payment_bill_reconcile.trade_type; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN payment_bill_reconcile.trade_type IS '交易类型';


--
-- Name: COLUMN payment_bill_reconcile.channel_status; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN payment_bill_reconcile.channel_status IS '渠道侧交易状态';


--
-- Name: COLUMN payment_bill_reconcile.channel_refund_status; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN payment_bill_reconcile.channel_refund_status IS '渠道侧退款状态';


--
-- Name: COLUMN payment_bill_reconcile.channel_amount; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN payment_bill_reconcile.channel_amount IS '渠道侧交易金额';


--
-- Name: COLUMN payment_bill_reconcile.channel_refund_amount; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN payment_bill_reconcile.channel_refund_amount IS '渠道侧退款金额';


--
-- Name: COLUMN payment_bill_reconcile.local_type; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN payment_bill_reconcile.local_type IS '本地业务类型，如 topup、subscription';


--
-- Name: COLUMN payment_bill_reconcile.local_id; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN payment_bill_reconcile.local_id IS '本地业务表主键ID';


--
-- Name: COLUMN payment_bill_reconcile.local_trade_no; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN payment_bill_reconcile.local_trade_no IS '本地订单号';


--
-- Name: COLUMN payment_bill_reconcile.local_payment_method; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN payment_bill_reconcile.local_payment_method IS '本地支付方式';


--
-- Name: COLUMN payment_bill_reconcile.local_status; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN payment_bill_reconcile.local_status IS '本地订单状态';


--
-- Name: COLUMN payment_bill_reconcile.local_amount; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN payment_bill_reconcile.local_amount IS '本地订单金额';


--
-- Name: COLUMN payment_bill_reconcile.local_create_time; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN payment_bill_reconcile.local_create_time IS '本地订单创建时间戳';


--
-- Name: COLUMN payment_bill_reconcile.local_complete_time; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN payment_bill_reconcile.local_complete_time IS '本地订单完成时间戳';


--
-- Name: COLUMN payment_bill_reconcile.reconcile_status; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN payment_bill_reconcile.reconcile_status IS '对账结果状态，如 matched、abnormal';


--
-- Name: COLUMN payment_bill_reconcile.reconcile_reason; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN payment_bill_reconcile.reconcile_reason IS '对账结果原因，如 amount_mismatch、local_not_found';


--
-- Name: COLUMN payment_bill_reconcile.remark; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN payment_bill_reconcile.remark IS '对账备注说明';


--
-- Name: COLUMN payment_bill_reconcile.created_at; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN payment_bill_reconcile.created_at IS '创建时间戳';


--
-- Name: COLUMN payment_bill_reconcile.updated_at; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN payment_bill_reconcile.updated_at IS '更新时间戳';


--
-- Name: COLUMN payment_bill_reconcile.reconcile_key; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN payment_bill_reconcile.reconcile_key IS '对账记录唯一键，本地单和渠道单边记录都靠它幂等更新';


--
-- Name: COLUMN payment_bill_reconcile.record_source; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN payment_bill_reconcile.record_source IS '记录来源，local=本地订单基准记录，channel=微信账单单边记录';


--
-- Name: payment_bill_reconcile_id_seq; Type: SEQUENCE; Schema: public; Owner: allrouter
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
-- Name: payment_bill_record; Type: TABLE; Schema: public; Owner: allrouter
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
    channel_trade_no character varying(64) DEFAULT ''::character varying,
    merchant_trade_no character varying(64) DEFAULT ''::character varying,
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



COMMENT ON TABLE payment_bill_record IS '通用支付渠道账单明细表';


--
-- Name: COLUMN payment_bill_record.id; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN payment_bill_record.id IS '主键ID';


--
-- Name: COLUMN payment_bill_record.channel_type; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN payment_bill_record.channel_type IS '支付渠道类型，如 wxpay、stripe';


--
-- Name: COLUMN payment_bill_record.bill_date; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN payment_bill_record.bill_date IS '账单日期，格式 YYYY-MM-DD';


--
-- Name: COLUMN payment_bill_record.file_path; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN payment_bill_record.file_path IS '本地保存的账单文件路径';


--
-- Name: COLUMN payment_bill_record.row_index; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN payment_bill_record.row_index IS '账单文件中的行号';


--
-- Name: COLUMN payment_bill_record.row_hash; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN payment_bill_record.row_hash IS '账单行内容哈希，用于去重';


--
-- Name: COLUMN payment_bill_record.trade_time; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN payment_bill_record.trade_time IS '渠道账单中的交易时间';


--
-- Name: COLUMN payment_bill_record.app_id; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN payment_bill_record.app_id IS '应用ID，例如微信公众账号ID';


--
-- Name: COLUMN payment_bill_record.mch_id; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN payment_bill_record.mch_id IS '商户号';


--
-- Name: COLUMN payment_bill_record.sub_mch_id; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN payment_bill_record.sub_mch_id IS '子商户号/特约商户号';


--
-- Name: COLUMN payment_bill_record.device_id; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN payment_bill_record.device_id IS '设备号';


--
-- Name: COLUMN payment_bill_record.channel_trade_no; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN payment_bill_record.channel_trade_no IS '支付渠道交易单号，例如微信订单号';


--
-- Name: COLUMN payment_bill_record.merchant_trade_no; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN payment_bill_record.merchant_trade_no IS '商户订单号';


--
-- Name: COLUMN payment_bill_record.payer_id; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN payment_bill_record.payer_id IS '支付用户标识，例如微信 openid';


--
-- Name: COLUMN payment_bill_record.trade_type; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN payment_bill_record.trade_type IS '交易类型';


--
-- Name: COLUMN payment_bill_record.trade_status; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN payment_bill_record.trade_status IS '交易状态';


--
-- Name: COLUMN payment_bill_record.refund_status; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN payment_bill_record.refund_status IS '退款状态';


--
-- Name: COLUMN payment_bill_record.refund_type; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN payment_bill_record.refund_type IS '退款类型';


--
-- Name: COLUMN payment_bill_record.currency; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN payment_bill_record.currency IS '货币类型';


--
-- Name: COLUMN payment_bill_record.bank; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN payment_bill_record.bank IS '付款银行';


--
-- Name: COLUMN payment_bill_record.total_amount; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN payment_bill_record.total_amount IS '总金额';


--
-- Name: COLUMN payment_bill_record.order_amount; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN payment_bill_record.order_amount IS '订单金额';


--
-- Name: COLUMN payment_bill_record.refund_amount; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN payment_bill_record.refund_amount IS '退款金额';


--
-- Name: COLUMN payment_bill_record.service_fee; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN payment_bill_record.service_fee IS '手续费';


--
-- Name: COLUMN payment_bill_record.rate; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN payment_bill_record.rate IS '费率';


--
-- Name: COLUMN payment_bill_record.rate_remark; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN payment_bill_record.rate_remark IS '费率备注';


--
-- Name: COLUMN payment_bill_record.goods_name; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN payment_bill_record.goods_name IS '商品名称';


--
-- Name: COLUMN payment_bill_record.package_data; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN payment_bill_record.package_data IS '商户数据包/附加信息';


--
-- Name: COLUMN payment_bill_record.channel_refund_no; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN payment_bill_record.channel_refund_no IS '支付渠道退款单号，例如微信退款单号';


--
-- Name: COLUMN payment_bill_record.merchant_refund_no; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN payment_bill_record.merchant_refund_no IS '商户退款单号';


--
-- Name: COLUMN payment_bill_record.enterprise_red_packet; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN payment_bill_record.enterprise_red_packet IS '企业红包金额';


--
-- Name: COLUMN payment_bill_record.enterprise_refund; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN payment_bill_record.enterprise_refund IS '企业红包退款金额';


--
-- Name: COLUMN payment_bill_record.apply_refund_amount; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN payment_bill_record.apply_refund_amount IS '申请退款金额';


--
-- Name: COLUMN payment_bill_record.raw_line; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN payment_bill_record.raw_line IS '账单原始整行文本';


--
-- Name: COLUMN payment_bill_record.raw_data_json; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN payment_bill_record.raw_data_json IS '账单原始字段 JSON';


--
-- Name: COLUMN payment_bill_record.created_at; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN payment_bill_record.created_at IS '创建时间戳';


--
-- Name: payment_bill_record_id_seq; Type: SEQUENCE; Schema: public; Owner: allrouter
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
-- Name: prefill_groups; Type: TABLE; Schema: public; Owner: allrouter
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


CREATE SEQUENCE prefill_groups_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;



ALTER SEQUENCE prefill_groups_id_seq OWNED BY prefill_groups.id;


--
-- Name: quota_data; Type: TABLE; Schema: public; Owner: allrouter
--

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


--
-- Name: redemptions; Type: TABLE; Schema: public; Owner: allrouter
--

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
    expired_time bigint
);



CREATE SEQUENCE redemptions_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;



ALTER SEQUENCE redemptions_id_seq OWNED BY redemptions.id;


--
-- Name: service_configs; Type: TABLE; Schema: public; Owner: allrouter
--

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
    wechat_cert_path character varying(255)
);


COMMENT ON COLUMN service_configs.wechat_serial_no IS '微信支付平台证书序列号';


--
-- Name: COLUMN service_configs.wechat_cert_path; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN service_configs.wechat_cert_path IS '微信支付平台证书路径';


--
-- Name: service_configs_id_seq; Type: SEQUENCE; Schema: public; Owner: allrouter
--

CREATE SEQUENCE service_configs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;



ALTER SEQUENCE service_configs_id_seq OWNED BY service_configs.id;


--
-- Name: setups; Type: TABLE; Schema: public; Owner: allrouter
--

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


--
-- Name: subscription_orders; Type: TABLE; Schema: public; Owner: allrouter
--

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
    original_money numeric(18,6) DEFAULT 0 NOT NULL
);


CREATE SEQUENCE subscription_orders_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE subscription_orders_id_seq OWNED BY subscription_orders.id;


--
-- Name: subscription_plans; Type: TABLE; Schema: public; Owner: allrouter
--

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
    stripe_price_cny_id character varying(128) DEFAULT ''::character varying
);



CREATE SEQUENCE subscription_plans_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;



ALTER SEQUENCE subscription_plans_id_seq OWNED BY subscription_plans.id;


--
-- Name: subscription_pre_consume_records; Type: TABLE; Schema: public; Owner: allrouter
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


--
-- Name: tasks; Type: TABLE; Schema: public; Owner: allrouter
--

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


ALTER SEQUENCE tasks_id_seq OWNED BY tasks.id;


--
-- Name: timezone_currency_map; Type: TABLE; Schema: public; Owner: allrouter
--

CREATE TABLE timezone_currency_map (
    timezone character varying(64) NOT NULL,
    currency character varying(3) NOT NULL,
    updated_at timestamp with time zone DEFAULT now()
);


CREATE TABLE tokens (
    id bigint NOT NULL,
    user_id bigint,
    key character(48),
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
    deleted_at timestamp with time zone
);


CREATE SEQUENCE tokens_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;



ALTER SEQUENCE tokens_id_seq OWNED BY tokens.id;


--
-- Name: top_ups; Type: TABLE; Schema: public; Owner: allrouter
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
    original_money numeric(18,6) DEFAULT 0 NOT NULL
);


CREATE SEQUENCE top_ups_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;



ALTER SEQUENCE top_ups_id_seq OWNED BY top_ups.id;


--
-- Name: topup_rebates; Type: TABLE; Schema: public; Owner: allrouter
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
    status text
);


COMMENT ON TABLE topup_rebates IS '邀请充值返利记录表';


--
-- Name: topup_rebates_id_seq; Type: SEQUENCE; Schema: public; Owner: allrouter
--

CREATE SEQUENCE topup_rebates_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE topup_rebates_id_seq OWNED BY topup_rebates.id;


--
-- Name: two_fa_backup_codes; Type: TABLE; Schema: public; Owner: allrouter
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


CREATE SEQUENCE two_fa_backup_codes_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE two_fa_backup_codes_id_seq OWNED BY two_fa_backup_codes.id;


--
-- Name: two_fas; Type: TABLE; Schema: public; Owner: allrouter
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


--
-- Name: user_oauth_bindings; Type: TABLE; Schema: public; Owner: allrouter
--

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


--
-- Name: user_subscriptions; Type: TABLE; Schema: public; Owner: allrouter
--

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


--
-- Name: users; Type: TABLE; Schema: public; Owner: allrouter
--

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
    phone_country_code character varying(8),
    phone_number character varying(20),
    timezone character varying(64),
    avatar character varying(255),
    signup_source character varying(64)
);


COMMENT ON COLUMN users.phone_country_code IS '手机号国家区号（E.164），如 +86';


--
-- Name: COLUMN users.phone_number; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN users.phone_number IS '手机号本地号码，不含国家区号，如 13800000000';


--
-- Name: COLUMN users.timezone; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN users.timezone IS '时区标识（IANA），如 Asia/Shanghai';


--
-- Name: COLUMN users.avatar; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN users.avatar IS '头像';


--
-- Name: COLUMN users.signup_source; Type: COMMENT; Schema: public; Owner: allrouter
--

COMMENT ON COLUMN users.signup_source IS '注册来源';


--
-- Name: users_id_seq; Type: SEQUENCE; Schema: public; Owner: allrouter
--

CREATE SEQUENCE users_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE users_id_seq OWNED BY users.id;


--
-- Name: vendors; Type: TABLE; Schema: public; Owner: allrouter
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

CREATE SEQUENCE vendors_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE vendors_id_seq OWNED BY vendors.id;


--
-- Name: admin_sessions id; Type: DEFAULT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY admin_sessions ALTER COLUMN id SET DEFAULT nextval('admin_sessions_id_seq'::regclass);


--
-- Name: admin_users id; Type: DEFAULT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY admin_users ALTER COLUMN id SET DEFAULT nextval('admin_users_id_seq'::regclass);


--
-- Name: channels id; Type: DEFAULT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY channels ALTER COLUMN id SET DEFAULT nextval('channels_id_seq'::regclass);


--
-- Name: checkins id; Type: DEFAULT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY checkins ALTER COLUMN id SET DEFAULT nextval('checkins_id_seq'::regclass);


--
-- Name: custom_oauth_providers id; Type: DEFAULT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY custom_oauth_providers ALTER COLUMN id SET DEFAULT nextval('custom_oauth_providers_id_seq'::regclass);


--
-- Name: epay_merchants id; Type: DEFAULT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY epay_merchants ALTER COLUMN id SET DEFAULT nextval('epay_merchants_id_seq'::regclass);


--
-- Name: login_audit_logs id; Type: DEFAULT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY login_audit_logs ALTER COLUMN id SET DEFAULT nextval('login_audit_logs_id_seq'::regclass);


--
-- Name: logs id; Type: DEFAULT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY logs ALTER COLUMN id SET DEFAULT nextval('logs_id_seq'::regclass);


--
-- Name: midjourneys id; Type: DEFAULT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY midjourneys ALTER COLUMN id SET DEFAULT nextval('midjourneys_id_seq'::regclass);


--
-- Name: models id; Type: DEFAULT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY models ALTER COLUMN id SET DEFAULT nextval('models_id_seq'::regclass);


--
-- Name: orders id; Type: DEFAULT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY orders ALTER COLUMN id SET DEFAULT nextval('orders_id_seq'::regclass);


--
-- Name: passkey_credentials id; Type: DEFAULT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY passkey_credentials ALTER COLUMN id SET DEFAULT nextval('passkey_credentials_id_seq'::regclass);


--
-- Name: prefill_groups id; Type: DEFAULT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY prefill_groups ALTER COLUMN id SET DEFAULT nextval('prefill_groups_id_seq'::regclass);


--
-- Name: quota_data id; Type: DEFAULT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY quota_data ALTER COLUMN id SET DEFAULT nextval('quota_data_id_seq'::regclass);


--
-- Name: redemptions id; Type: DEFAULT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY redemptions ALTER COLUMN id SET DEFAULT nextval('redemptions_id_seq'::regclass);


--
-- Name: service_configs id; Type: DEFAULT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY service_configs ALTER COLUMN id SET DEFAULT nextval('service_configs_id_seq'::regclass);


--
-- Name: setups id; Type: DEFAULT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY setups ALTER COLUMN id SET DEFAULT nextval('setups_id_seq'::regclass);


--
-- Name: subscription_orders id; Type: DEFAULT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY subscription_orders ALTER COLUMN id SET DEFAULT nextval('subscription_orders_id_seq'::regclass);


--
-- Name: subscription_plans id; Type: DEFAULT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY subscription_plans ALTER COLUMN id SET DEFAULT nextval('subscription_plans_id_seq'::regclass);


--
-- Name: subscription_pre_consume_records id; Type: DEFAULT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY subscription_pre_consume_records ALTER COLUMN id SET DEFAULT nextval('subscription_pre_consume_records_id_seq'::regclass);


--
-- Name: tasks id; Type: DEFAULT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY tasks ALTER COLUMN id SET DEFAULT nextval('tasks_id_seq'::regclass);


--
-- Name: tokens id; Type: DEFAULT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY tokens ALTER COLUMN id SET DEFAULT nextval('tokens_id_seq'::regclass);


--
-- Name: top_ups id; Type: DEFAULT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY top_ups ALTER COLUMN id SET DEFAULT nextval('top_ups_id_seq'::regclass);


--
-- Name: topup_rebates id; Type: DEFAULT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY topup_rebates ALTER COLUMN id SET DEFAULT nextval('topup_rebates_id_seq'::regclass);


--
-- Name: two_fa_backup_codes id; Type: DEFAULT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY two_fa_backup_codes ALTER COLUMN id SET DEFAULT nextval('two_fa_backup_codes_id_seq'::regclass);


--
-- Name: two_fas id; Type: DEFAULT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY two_fas ALTER COLUMN id SET DEFAULT nextval('two_fas_id_seq'::regclass);


--
-- Name: user_oauth_bindings id; Type: DEFAULT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY user_oauth_bindings ALTER COLUMN id SET DEFAULT nextval('user_oauth_bindings_id_seq'::regclass);


--
-- Name: user_subscriptions id; Type: DEFAULT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY user_subscriptions ALTER COLUMN id SET DEFAULT nextval('user_subscriptions_id_seq'::regclass);


--
-- Name: users id; Type: DEFAULT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY users ALTER COLUMN id SET DEFAULT nextval('users_id_seq'::regclass);


--
-- Name: vendors id; Type: DEFAULT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY vendors ALTER COLUMN id SET DEFAULT nextval('vendors_id_seq'::regclass);


INSERT INTO abilities ("group", model, channel_id, enabled, priority, weight, tag) VALUES ('Claude Code', 'deepseek-v4-pro', 34, true, 0, 0, '');
INSERT INTO abilities ("group", model, channel_id, enabled, priority, weight, tag) VALUES ('Claude Code', 'deepseek-v4-flash', 34, true, 0, 0, '');
INSERT INTO abilities ("group", model, channel_id, enabled, priority, weight, tag) VALUES ('codex CLI专用', 'gemini-3.1-pro-preview', 1004, true, 0, 0, '');
INSERT INTO abilities ("group", model, channel_id, enabled, priority, weight, tag) VALUES ('codex CLI专用', 'gemini-3-pro-preview', 1004, true, 0, 0, '');
INSERT INTO abilities ("group", model, channel_id, enabled, priority, weight, tag) VALUES ('google cli', 'gemini-3.1-pro-preview', 31, true, 0, 0, '');
INSERT INTO abilities ("group", model, channel_id, enabled, priority, weight, tag) VALUES ('google cli', 'gemini-3-flash-preview', 31, true, 0, 0, '');
INSERT INTO abilities ("group", model, channel_id, enabled, priority, weight, tag) VALUES ('google cli', 'gemini-3-pro-preview', 31, true, 0, 0, '');
INSERT INTO abilities ("group", model, channel_id, enabled, priority, weight, tag) VALUES ('google cli', 'gemini-2.5-flash', 31, true, 0, 0, '');
INSERT INTO abilities ("group", model, channel_id, enabled, priority, weight, tag) VALUES ('google cli', 'gemini-2.5-pro', 31, true, 0, 0, '');
INSERT INTO abilities ("group", model, channel_id, enabled, priority, weight, tag) VALUES ('OpenRouter', 'anthropic/claude-sonnet-4.6', 32, true, 0, 0, '');
INSERT INTO abilities ("group", model, channel_id, enabled, priority, weight, tag) VALUES ('OpenRouter', 'qwen/qwen3.6-plus', 32, true, 0, 0, '');
INSERT INTO abilities ("group", model, channel_id, enabled, priority, weight, tag) VALUES ('default', 'deepseek-v4-flash', 35, true, 0, 0, '');
INSERT INTO abilities ("group", model, channel_id, enabled, priority, weight, tag) VALUES ('default', 'deepseek-v4-pro', 35, true, 0, 0, '');
INSERT INTO abilities ("group", model, channel_id, enabled, priority, weight, tag) VALUES ('Codex CLI', 'gpt-5.4', 1001, true, 0, 0, '');
INSERT INTO abilities ("group", model, channel_id, enabled, priority, weight, tag) VALUES ('Codex CLI', 'gpt-5.4-mini', 1001, true, 0, 0, '');
INSERT INTO abilities ("group", model, channel_id, enabled, priority, weight, tag) VALUES ('Codex CLI', 'gpt-5.2', 1001, true, 0, 0, '');
INSERT INTO abilities ("group", model, channel_id, enabled, priority, weight, tag) VALUES ('Codex CLI', 'gpt-5.3-codex', 1001, true, 0, 0, '');
INSERT INTO abilities ("group", model, channel_id, enabled, priority, weight, tag) VALUES ('default', 'gemma-4-31b-it', 27, true, 0, 0, 'AITHER');
INSERT INTO abilities ("group", model, channel_id, enabled, priority, weight, tag) VALUES ('default', 'minimax-m2.7', 27, true, 0, 0, 'AITHER');
INSERT INTO abilities ("group", model, channel_id, enabled, priority, weight, tag) VALUES ('codex CLI专用', 'gpt-5.4', 1002, true, 0, 0, '');
INSERT INTO abilities ("group", model, channel_id, enabled, priority, weight, tag) VALUES ('codex CLI专用', 'gpt-5', 1002, true, 0, 0, '');
INSERT INTO abilities ("group", model, channel_id, enabled, priority, weight, tag) VALUES ('codex CLI专用', 'gpt-5.4', 1003, true, 0, 0, '');
INSERT INTO abilities ("group", model, channel_id, enabled, priority, weight, tag) VALUES ('codex CLI专用', 'gpt-5', 1003, true, 0, 0, '');
INSERT INTO abilities ("group", model, channel_id, enabled, priority, weight, tag) VALUES ('codex CLI专用', 'gpt-5.4', 1005, true, 0, 0, '');
INSERT INTO abilities ("group", model, channel_id, enabled, priority, weight, tag) VALUES ('codex CLI专用', 'gpt-5', 1005, true, 0, 0, '');
INSERT INTO abilities ("group", model, channel_id, enabled, priority, weight, tag) VALUES ('codex CLI专用', 'gpt-5.4', 1007, true, 0, 0, '');
INSERT INTO abilities ("group", model, channel_id, enabled, priority, weight, tag) VALUES ('codex CLI专用', 'gpt-5', 1007, true, 0, 0, '');


INSERT INTO admin_users (id, username, password_hash, password_changed, created_at, updated_at) VALUES (1, 'admin', '$2a$12$EXWCpXd6TVZKe2UTVdMOAO1HCmUbknRZKC3f22AM7VF9Xj5nXjWtK', false, '2026-03-12 18:02:37.145913+08', '2026-03-12 18:02:37.145913+08');


INSERT INTO channels (id, type, key, open_ai_organization, test_model, status, name, weight, created_time, test_time, response_time, base_url, other, balance, balance_updated_time, models, "group", used_quota, model_mapping, status_code_mapping, priority, auto_ban, other_info, tag, setting, param_override, header_override, remark, channel_info, settings) VALUES (35, 1, '', '', '', 1, 'deepseek v4', 0, 1777544176, 1777544233, 1100, 'https://allrouter.ai', '', 0, 0, 'deepseek-v4-flash,deepseek-v4-pro', 'default', 5770, '', '', 0, 1, '', '', '{"force_format":false,"thinking_to_content":false,"proxy":"","pass_through_body_enabled":false,"system_prompt":"","system_prompt_override":false}', '', NULL, NULL, '{"is_multi_key":false,"multi_key_size":0,"multi_key_status_list":null,"multi_key_polling_index":0,"multi_key_mode":""}', '{"allow_service_tier":false,"disable_store":false,"allow_safety_identifier":false,"allow_include_obfuscation":false,"upstream_model_update_check_enabled":false,"upstream_model_update_auto_sync_enabled":false,"upstream_model_update_ignored_models":[],"upstream_model_update_last_detected_models":[],"upstream_model_update_last_check_time":0}');
INSERT INTO channels (id, type, key, open_ai_organization, test_model, status, name, weight, created_time, test_time, response_time, base_url, other, balance, balance_updated_time, models, "group", used_quota, model_mapping, status_code_mapping, priority, auto_ban, other_info, tag, setting, param_override, header_override, remark, channel_info, settings) VALUES (31, 1, '', '', '', 1, 'googlebusiness2api', 0, 1775205526, 1775630129, 76497, 'https://allrouter.ai', '', 0, 0, 'gemini-3.1-pro-preview,gemini-3-flash-preview,gemini-3-pro-preview,gemini-2.5-flash,gemini-2.5-pro', 'google cli', 1480748, '', '', 0, 1, '', '', '{"force_format":false,"thinking_to_content":false,"proxy":"","pass_through_body_enabled":false,"system_prompt":"","system_prompt_override":false}', '', NULL, NULL, '{"is_multi_key":false,"multi_key_size":0,"multi_key_status_list":null,"multi_key_polling_index":0,"multi_key_mode":"random"}', '{"allow_service_tier":false,"disable_store":false,"allow_safety_identifier":false,"allow_include_obfuscation":false,"upstream_model_update_check_enabled":false,"upstream_model_update_auto_sync_enabled":false,"upstream_model_update_ignored_models":[],"upstream_model_update_last_detected_models":[],"upstream_model_update_last_check_time":0}');
INSERT INTO channels (id, type, key, open_ai_organization, test_model, status, name, weight, created_time, test_time, response_time, base_url, other, balance, balance_updated_time, models, "group", used_quota, model_mapping, status_code_mapping, priority, auto_ban, other_info, tag, setting, param_override, header_override, remark, channel_info, settings) VALUES (27, 1, '', NULL, '', 1, 'Fedimoss', 0, 1773279064, 1776678607, 978, 'https://allrouter.ai', '', 0, 0, 'gemma-4-31b-it,minimax-m2.7', 'default', 19162895, '', '', 0, 1, '', 'AITHER', '{"force_format":false,"thinking_to_content":false,"proxy":"","pass_through_body_enabled":false,"system_prompt":"","system_prompt_override":false}', '', NULL, NULL, '{"is_multi_key":false,"multi_key_size":0,"multi_key_status_list":null,"multi_key_polling_index":0,"multi_key_mode":"random"}', '{"allow_service_tier":false,"disable_store":false,"allow_safety_identifier":false,"allow_include_obfuscation":false,"upstream_model_update_check_enabled":false,"upstream_model_update_auto_sync_enabled":false,"upstream_model_update_ignored_models":[],"upstream_model_update_last_detected_models":[],"upstream_model_update_last_check_time":0}');
INSERT INTO channels (id, type, key, open_ai_organization, test_model, status, name, weight, created_time, test_time, response_time, base_url, other, balance, balance_updated_time, models, "group", used_quota, model_mapping, status_code_mapping, priority, auto_ban, other_info, tag, setting, param_override, header_override, remark, channel_info, settings) VALUES (1001, 1, '', '', '', 1, 'cliProxyApi_codexcli', 0, 1773987527, 1776335875, 1005, 'https://allrouter.ai', '', 0, 0, 'gpt-5.4,gpt-5.4-mini,gpt-5.2,gpt-5.3-codex', 'Codex CLI', 606842112, '', '', 0, 1, '', '', '{"force_format":false,"thinking_to_content":false,"proxy":"","pass_through_body_enabled":false,"system_prompt":"","system_prompt_override":false}', '', '', NULL, '{"is_multi_key":true,"multi_key_size":1,"multi_key_status_list":null,"multi_key_polling_index":0,"multi_key_mode":"polling"}', '{"allow_service_tier":false,"disable_store":false,"allow_safety_identifier":false,"allow_include_obfuscation":false,"upstream_model_update_check_enabled":false,"upstream_model_update_auto_sync_enabled":false,"upstream_model_update_ignored_models":[],"upstream_model_update_last_detected_models":[],"upstream_model_update_last_check_time":0}');
INSERT INTO channels (id, type, key, open_ai_organization, test_model, status, name, weight, created_time, test_time, response_time, base_url, other, balance, balance_updated_time, models, "group", used_quota, model_mapping, status_code_mapping, priority, auto_ban, other_info, tag, setting, param_override, header_override, remark, channel_info, settings) VALUES (1004, 1, '', '', '', 1, 'cliProxyApi_geminicli', 0, 1773987527, 1773995072, 977, 'https://allrouter.ai', '', 0, 0, 'gemini-3.1-pro-preview,gemini-3-pro-preview', 'codex CLI专用', 17387, '', '', 0, 1, '', '', '{"force_format":false,"thinking_to_content":false,"proxy":"","pass_through_body_enabled":false,"system_prompt":"","system_prompt_override":false}', '', NULL, NULL, '{"is_multi_key":true,"multi_key_size":1,"multi_key_status_list":null,"multi_key_polling_index":0,"multi_key_mode":"polling"}', '{"allow_service_tier":false,"disable_store":false,"allow_safety_identifier":false,"allow_include_obfuscation":false,"upstream_model_update_check_enabled":false,"upstream_model_update_auto_sync_enabled":false,"upstream_model_update_ignored_models":[],"upstream_model_update_last_detected_models":[],"upstream_model_update_last_check_time":0}');
INSERT INTO channels (id, type, key, open_ai_organization, test_model, status, name, weight, created_time, test_time, response_time, base_url, other, balance, balance_updated_time, models, "group", used_quota, model_mapping, status_code_mapping, priority, auto_ban, other_info, tag, setting, param_override, header_override, remark, channel_info, settings) VALUES (1002, 1, '', '', '', 1, 'cliProxyApi_claudecodecli', 0, 1773987527, 1773995072, 977, 'https://allrouter.ai', '', 0, 0, 'gpt-5.4,gpt-5', 'codex CLI专用', 17387, '', '', 0, 1, '', '', '{"force_format":false,"thinking_to_content":false,"proxy":"","pass_through_body_enabled":false,"system_prompt":"","system_prompt_override":false}', '', NULL, NULL, '{"is_multi_key":true,"multi_key_size":1,"multi_key_status_list":null,"multi_key_polling_index":0,"multi_key_mode":"polling"}', '{"allow_service_tier":false,"disable_store":false,"allow_safety_identifier":false,"allow_include_obfuscation":false,"upstream_model_update_check_enabled":false,"upstream_model_update_auto_sync_enabled":false,"upstream_model_update_ignored_models":[],"upstream_model_update_last_detected_models":[],"upstream_model_update_last_check_time":0}');
INSERT INTO channels (id, type, key, open_ai_organization, test_model, status, name, weight, created_time, test_time, response_time, base_url, other, balance, balance_updated_time, models, "group", used_quota, model_mapping, status_code_mapping, priority, auto_ban, other_info, tag, setting, param_override, header_override, remark, channel_info, settings) VALUES (1003, 1, '', '', '', 1, 'cliProxyApi_antigravitycli', 0, 1773987527, 1773995072, 977, 'https://allrouter.ai', '', 0, 0, 'gpt-5.4,gpt-5', 'codex CLI专用', 17387, '', '', 0, 1, '', '', '{"force_format":false,"thinking_to_content":false,"proxy":"","pass_through_body_enabled":false,"system_prompt":"","system_prompt_override":false}', '', NULL, NULL, '{"is_multi_key":true,"multi_key_size":1,"multi_key_status_list":null,"multi_key_polling_index":0,"multi_key_mode":"polling"}', '{"allow_service_tier":false,"disable_store":false,"allow_safety_identifier":false,"allow_include_obfuscation":false,"upstream_model_update_check_enabled":false,"upstream_model_update_auto_sync_enabled":false,"upstream_model_update_ignored_models":[],"upstream_model_update_last_detected_models":[],"upstream_model_update_last_check_time":0}');
INSERT INTO channels (id, type, key, open_ai_organization, test_model, status, name, weight, created_time, test_time, response_time, base_url, other, balance, balance_updated_time, models, "group", used_quota, model_mapping, status_code_mapping, priority, auto_ban, other_info, tag, setting, param_override, header_override, remark, channel_info, settings) VALUES (1005, 1, '', '', '', 1, 'cliProxyApi_kimicli', 0, 1773987527, 1773995072, 977, 'https://allrouter.ai', '', 0, 0, 'gpt-5.4,gpt-5', 'codex CLI专用', 17387, '', '', 0, 1, '', '', '{"force_format":false,"thinking_to_content":false,"proxy":"","pass_through_body_enabled":false,"system_prompt":"","system_prompt_override":false}', '', NULL, NULL, '{"is_multi_key":true,"multi_key_size":1,"multi_key_status_list":null,"multi_key_polling_index":0,"multi_key_mode":"polling"}', '{"allow_service_tier":false,"disable_store":false,"allow_safety_identifier":false,"allow_include_obfuscation":false,"upstream_model_update_check_enabled":false,"upstream_model_update_auto_sync_enabled":false,"upstream_model_update_ignored_models":[],"upstream_model_update_last_detected_models":[],"upstream_model_update_last_check_time":0}');
INSERT INTO channels (id, type, key, open_ai_organization, test_model, status, name, weight, created_time, test_time, response_time, base_url, other, balance, balance_updated_time, models, "group", used_quota, model_mapping, status_code_mapping, priority, auto_ban, other_info, tag, setting, param_override, header_override, remark, channel_info, settings) VALUES (1007, 1, '', '', '', 1, 'cliProxyApi_iflowcli', 0, 1773987527, 1773995072, 977, 'https://allrouter.ai', '', 0, 0, 'gpt-5.4,gpt-5', 'codex CLI专用', 17387, '', '', 0, 1, '', '', '{"force_format":false,"thinking_to_content":false,"proxy":"","pass_through_body_enabled":false,"system_prompt":"","system_prompt_override":false}', '', NULL, NULL, '{"is_multi_key":true,"multi_key_size":1,"multi_key_status_list":null,"multi_key_polling_index":0,"multi_key_mode":"polling"}', '{"allow_service_tier":false,"disable_store":false,"allow_safety_identifier":false,"allow_include_obfuscation":false,"upstream_model_update_check_enabled":false,"upstream_model_update_auto_sync_enabled":false,"upstream_model_update_ignored_models":[],"upstream_model_update_last_detected_models":[],"upstream_model_update_last_check_time":0}');
INSERT INTO channels (id, type, key, open_ai_organization, test_model, status, name, weight, created_time, test_time, response_time, base_url, other, balance, balance_updated_time, models, "group", used_quota, model_mapping, status_code_mapping, priority, auto_ban, other_info, tag, setting, param_override, header_override, remark, channel_info, settings) VALUES (34, 14, '', NULL, '', 1, 'Fedimoss_compshare', 0, 1777011167, 1777540896, 839, 'https://allrouter.ai', '', 0, 0, 'deepseek-v4-pro,deepseek-v4-flash', 'Claude Code', 515194, '', '', 0, 1, '', '', '{"force_format":false,"thinking_to_content":false,"proxy":"","pass_through_body_enabled":false,"system_prompt":"","system_prompt_override":false}', '', NULL, NULL, '{"is_multi_key":false,"multi_key_size":0,"multi_key_status_list":null,"multi_key_polling_index":0,"multi_key_mode":"random"}', '{"allow_service_tier":false,"allow_inference_geo":false,"allow_speed":false,"claude_beta_query":false,"upstream_model_update_check_enabled":false,"upstream_model_update_auto_sync_enabled":false,"upstream_model_update_ignored_models":[],"upstream_model_update_last_detected_models":[],"upstream_model_update_last_check_time":0}');
INSERT INTO channels (id, type, key, open_ai_organization, test_model, status, name, weight, created_time, test_time, response_time, base_url, other, balance, balance_updated_time, models, "group", used_quota, model_mapping, status_code_mapping, priority, auto_ban, other_info, tag, setting, param_override, header_override, remark, channel_info, settings) VALUES (32, 1, '', '', '', 1, 'OpenRouter', 0, 1775613444, 1776336256, 2967, 'https://allrouter.ai', '', 0, 0, 'anthropic/claude-sonnet-4.6,qwen/qwen3.6-plus', 'OpenRouter', 27651477, '', '', 0, 1, '', '', '{"force_format":false,"thinking_to_content":false,"proxy":"","pass_through_body_enabled":false,"system_prompt":"","system_prompt_override":false}', '', NULL, NULL, '{"is_multi_key":false,"multi_key_size":0,"multi_key_status_list":null,"multi_key_polling_index":0,"multi_key_mode":"random"}', '{"allow_service_tier":false,"disable_store":false,"allow_safety_identifier":false,"allow_include_obfuscation":false,"upstream_model_update_check_enabled":false,"upstream_model_update_auto_sync_enabled":false,"upstream_model_update_ignored_models":[],"upstream_model_update_last_detected_models":[],"upstream_model_update_last_check_time":0}');



INSERT INTO currency_stripe_config (currency, stripe_price_id, unit_price, symbol, updated_at) VALUES ('USD', 'price_1TBWPEGfkXZHBVWZywLn2axf', 1.000000, '$', '2026-04-27 13:15:35.539079+08');
INSERT INTO currency_stripe_config (currency, stripe_price_id, unit_price, symbol, updated_at) VALUES ('CNY', 'price_1TPmQvGfkXZHBVWZUR6yWhjQ', 7.300000, '¥', '2026-04-27 13:15:35.542171+08');





INSERT INTO epay_merchants (id, pid, name, key, active, created_at, updated_at) VALUES (1, '1001', 'sjpay', '5a1e4dd49face8a69ba935e1787b5401b5abb514bac0acdff27761127f938c99', true, '2026-03-12 18:12:31.978717+08', '2026-03-12 18:12:31.978717+08');


INSERT INTO models (id, model_name, description, icon, tags, vendor_id, endpoints, status, sync_official, created_time, updated_time, deleted_at, name_rule) VALUES (1, 'gpt-4', '', 'OpenAI', '', 1, '[]', 1, 1, 1771906008, 1771906027, NULL, 0);
INSERT INTO models (id, model_name, description, icon, tags, vendor_id, endpoints, status, sync_official, created_time, updated_time, deleted_at, name_rule) VALUES (2, 'Doubao-1.8', '', '', '', 2, '{
  "ep-20251219142058-9g5cq": "Doubao-1.8"
}', 1, 1, 1771911373, 1771912172, '2026-02-24 13:50:14.35909+08', 0);
INSERT INTO models (id, model_name, description, icon, tags, vendor_id, endpoints, status, sync_official, created_time, updated_time, deleted_at, name_rule) VALUES (3, 'doubao-1.8', '', '', '', 2, '', 1, 1, 1771912198, 1771912662, '2026-02-24 13:58:03.253988+08', 2);
INSERT INTO models (id, model_name, description, icon, tags, vendor_id, endpoints, status, sync_official, created_time, updated_time, deleted_at, name_rule) VALUES (6, 'Doubao-1.8', '', '', '', 0, '', 1, 1, 1772256169, 1772256169, '2026-02-28 13:22:53.587714+08', 0);
INSERT INTO models (id, model_name, description, icon, tags, vendor_id, endpoints, status, sync_official, created_time, updated_time, deleted_at, name_rule) VALUES (7, 'claude-opus-4-6', '', '', '', 4, '{
  "anthropic": "anthropic"
}', 1, 1, 1772257783, 1772257863, NULL, 0);
INSERT INTO models (id, model_name, description, icon, tags, vendor_id, endpoints, status, sync_official, created_time, updated_time, deleted_at, name_rule) VALUES (8, 'claude-sonnet-4-6', '', '', '', 4, '{
  "anthropic": "anthropic"
}', 1, 1, 1772414195, 1772521900, NULL, 0);
INSERT INTO models (id, model_name, description, icon, tags, vendor_id, endpoints, status, sync_official, created_time, updated_time, deleted_at, name_rule) VALUES (9, 'MiniMax-M2.5', '', 'Minimax', '', 0, '{
  "MinMax": "MinMax"
}', 0, 1, 1772531909, 1772532003, NULL, 0);
INSERT INTO models (id, model_name, description, icon, tags, vendor_id, endpoints, status, sync_official, created_time, updated_time, deleted_at, name_rule) VALUES (5, 'DeepSeek-V3.2', '', '', '', 3, '{
  "deepseek": "deepseek"
}', 0, 1, 1772250841, 1772257876, NULL, 0);
INSERT INTO models (id, model_name, description, icon, tags, vendor_id, endpoints, status, sync_official, created_time, updated_time, deleted_at, name_rule) VALUES (4, 'ep-20251219142058-9g5cq', '', '', '', 2, '{
  "字节跳动": "字节跳动"
}', 0, 1, 1771912723, 1771912819, NULL, 0);
INSERT INTO models (id, model_name, description, icon, tags, vendor_id, endpoints, status, sync_official, created_time, updated_time, deleted_at, name_rule) VALUES (11, 'google/gemma-4-26b-a4b-it:free', '', '', '', 13, '["openai"]', 1, 1, 1775619037, 1775619081, NULL, 0);
INSERT INTO models (id, model_name, description, icon, tags, vendor_id, endpoints, status, sync_official, created_time, updated_time, deleted_at, name_rule) VALUES (12, 'google/gemma-4-31b-it:free', '', '', '', 13, '', 1, 1, 1775619101, 1775619101, NULL, 0);
INSERT INTO models (id, model_name, description, icon, tags, vendor_id, endpoints, status, sync_official, created_time, updated_time, deleted_at, name_rule) VALUES (13, 'qwen/qwen3.6-plus', '', '', '', 13, '', 1, 1, 1775624606, 1775624606, NULL, 0);
INSERT INTO models (id, model_name, description, icon, tags, vendor_id, endpoints, status, sync_official, created_time, updated_time, deleted_at, name_rule) VALUES (14, 'anthropic/claude-sonnet-4.6', '', '', '', 13, '', 1, 1, 1775624723, 1775624723, NULL, 0);
INSERT INTO models (id, model_name, description, icon, tags, vendor_id, endpoints, status, sync_official, created_time, updated_time, deleted_at, name_rule) VALUES (15, 'qwen3.5-plus', '', '', '', 8, '', 1, 1, 1775704992, 1775704992, '2026-04-09 12:30:57.816434+08', 0);
INSERT INTO models (id, model_name, description, icon, tags, vendor_id, endpoints, status, sync_official, created_time, updated_time, deleted_at, name_rule) VALUES (10, 'gemma-4-31b-it', 'Google最新Gemma开源模型系列，Fedimoss推理优化支持
Google Gemma Open Source Series Models, Inference Optimization Powered by Fedimoss AI', '', 'Google开源模型，Fedimoss推理优化', 12, '["openai"]', 1, 1, 1775618986, 1776413658, NULL, 0);
INSERT INTO models (id, model_name, description, icon, tags, vendor_id, endpoints, status, sync_official, created_time, updated_time, deleted_at, name_rule) VALUES (16, 'minimax-m2.7', 'Fedimoss推理优化支持
Inference Optimization Powered by Fedimoss AI', 'Minimax', '', 12, '[]', 1, 1, 1776736800, 1776853719, NULL, 0);
INSERT INTO models (id, model_name, description, icon, tags, vendor_id, endpoints, status, sync_official, created_time, updated_time, deleted_at, name_rule) VALUES (17, 'qwen3.6-plus', '', 'Qwen.Color', '', 14, '[]', 1, 1, 1777338265, 1777339891, NULL, 0);


--
-- Data for Name: options; Type: TABLE DATA; Schema: public; Owner: allrouter
--

INSERT INTO options (key, value) VALUES ('DemoSiteEnabled', 'false');
INSERT INTO options (key, value) VALUES ('SelfUseModeEnabled', 'false');
INSERT INTO options (key, value) VALUES ('SystemName', 'AllRouter.AI');
INSERT INTO options (key, value) VALUES ('general_setting.docs_link', 'docs');
INSERT INTO options (key, value) VALUES ('PayMethods', '[
  {
		"color": "rgba(var(--semi-green-5), 1)",
		"name": "微信",
		"type": "wxpay"
	},
	{
		"color": "rgba(var(--semi-green-5), 1)",
		"name": "Stripe",
		"type": "stripe"
	}
]');
INSERT INTO options (key, value) VALUES ('USDExchangeRate', '7.30');
INSERT INTO options (key, value) VALUES ('general_setting.quota_display_type', 'USD');
INSERT INTO options (key, value) VALUES ('QuotaForNewUser', '1000000');
INSERT INTO options (key, value) VALUES ('StripePromotionCodesEnabled', 'false');
INSERT INTO options (key, value) VALUES ('EpayKey', 'xxxxx');
INSERT INTO options (key, value) VALUES ('ModelRequestRateLimitDurationMinutes', '60');
INSERT INTO options (key, value) VALUES ('ModelRequestRateLimitEnabled', 'true');
INSERT INTO options (key, value) VALUES ('ModelRequestRateLimitCount', '1000');
INSERT INTO options (key, value) VALUES ('ModelRequestRateLimitSuccessCount', '1000');
INSERT INTO options (key, value) VALUES ('StripeWebhookSecret', 'xxxxx');
INSERT INTO options (key, value) VALUES ('PayAddress', 'https://allrouter.shengjian.net/epay-api');
INSERT INTO options (key, value) VALUES ('ServerAddress', 'https://allrouter.ai');
INSERT INTO options (key, value) VALUES ('StripeApiSecret', 'xxxxx');
INSERT INTO options (key, value) VALUES ('EpayId', '1001');
INSERT INTO options (key, value) VALUES ('Price', '1');
INSERT INTO options (key, value) VALUES ('StripePriceId', 'xxxxx');
INSERT INTO options (key, value) VALUES ('MinTopUp', '1');
INSERT INTO options (key, value) VALUES ('CustomCallbackAddress', 'https://allrouter.ai');
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
  "gemini-3.1-pro-preview": 1,
  "gemini-3.1-pro-preview-customtools": 1,
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
  "zai-org/GLM-4.5-FP8": 0.8,
  "qwen3.6-plus": 0.329
}');
INSERT INTO options (key, value) VALUES ('GroupRatio', '{
  "default": 1,
  "svip": 1,
  "vip": 1,
  "OpenRouter": 1.5,
  "Codex CLI": 0.5,
  "google cli": 0.5,
  "Claude Code": 0.5
}');
INSERT INTO options (key, value) VALUES ('UserUsableGroups', '{
  "default": "默认分组",
  "svip": "svip分组",
  "vip": "vip分组",
  "OpenRouter": "OpenRouter账号池",
  "Codex CLI": "自建号池，满血，无场景限制",
  "google cli": "gemini模型的使用",
  "Claude Code": "Claude Code专用"
}');
INSERT INTO options (key, value) VALUES ('ModelPrice', '{
  "black-forest-labs/flux-1.1-pro": 0.04,
  "dall-e-3": 0.04,
  "gemini-2.5-flash-image": 0.2,
  "gemini-3-pro-image-preview": 0.95,
  "gemini-3.1-flash-image-preview": 0.5,
  "gpt-4-gizmo-*": 0.1,
  "gpt-4o-mini-tts": 0.3,
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
INSERT INTO options (key, value) VALUES ('CacheRatio', '{
  "claude-3-5-haiku-20241022": 0.05,
  "claude-3-5-sonnet-20240620": 0.1,
  "claude-3-5-sonnet-20241022": 0.1,
  "claude-3-7-sonnet-20250219": 0.1,
  "claude-3-7-sonnet-20250219-thinking": 0.1,
  "claude-3-haiku-20240307": 0.1,
  "claude-3-opus-20240229": 0.1,
  "claude-3-sonnet-20240229": 0.1,
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
  "gemini-3.1-pro-preview": 0.1,
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
  "gpt-5.4-high": 0.05,
  "gpt-5.4-high-openai-compact": 0.05,
  "gpt-5.4-low": 0.05,
  "gpt-5.4-low-openai-compact": 0.05,
  "gpt-5.4-medium": 0.05,
  "gpt-5.4-medium-openai-compact": 0.05,
  "gpt-5.4-mini": 0.05,
  "gpt-5.4-openai-compact": 0.05,
  "gpt-5.4-xhigh": 0.05,
  "gpt-5.4-xhigh-openai-compact": 0.05,
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
INSERT INTO options (key, value) VALUES ('Logo', 'https://ai.shengjian.net/coze/static/upload/agentLogo/20260420093900059070012611301892.png');
INSERT INTO options (key, value) VALUES ('AudioRatio', '{
  "gpt-4o-audio-preview": 16,
  "gpt-4o-mini-audio-preview": 0,
  "gpt-4o-mini-realtime-preview": 16.67,
  "gpt-4o-realtime-preview": 8
}');
INSERT INTO options (key, value) VALUES ('AudioCompletionRatio', '{}');
INSERT INTO options (key, value) VALUES ('StripeUnitPrice', '1');
INSERT INTO options (key, value) VALUES ('StripeMinTopUp', '1');
INSERT INTO options (key, value) VALUES ('CreateCacheRatio', '{
  "claude-3-5-haiku-20241022": 0.625,
  "claude-3-5-sonnet-20240620": 1.25,
  "claude-3-5-sonnet-20241022": 1.25,
  "claude-3-7-sonnet-20250219": 1.25,
  "claude-3-7-sonnet-20250219-thinking": 1.25,
  "claude-3-haiku-20240307": 1.25,
  "claude-3-opus-20240229": 1.25,
  "claude-3-sonnet-20240229": 1.25,
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
  "claude-sonnet-4-20250514": 0.0411,
  "claude-sonnet-4-20250514-thinking": 0.0411,
  "claude-sonnet-4-5-20250929": 0.0411,
  "claude-sonnet-4-5-20250929-thinking": 0.0411,
  "claude-sonnet-4-6": 0.0411,
  "claude-sonnet-4-6-thinking": 0.0411,
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
  "gpt-5.4-xhigh-openai-compact": 0.171232
}');
INSERT INTO options (key, value) VALUES ('QuotaForInviter', '500000');
INSERT INTO options (key, value) VALUES ('CLIProxyAPIPassword', 'sj@cli@2026');
INSERT INTO options (key, value) VALUES ('CLIServerAddress', 'http://172.31.39.126:8317');
INSERT INTO options (key, value) VALUES ('console_setting.api_info', '[{"id":1,"url":"https://allrouter.ai/","description":"稳定","route":"主线路","color":"blue"}]');
INSERT INTO options (key, value) VALUES ('EmailVerificationEnabled', 'true');
INSERT INTO options (key, value) VALUES ('DrawingEnabled', 'true');
INSERT INTO options (key, value) VALUES ('SMTPSSLEnabled', 'true');
INSERT INTO options (key, value) VALUES ('SMTPFrom', 'support@allrouter.ai');
INSERT INTO options (key, value) VALUES ('SMTPAccount', 'resend');
INSERT INTO options (key, value) VALUES ('SMTPToken', 'xxxxx');
INSERT INTO options (key, value) VALUES ('SMTPServer', 'smtp.resend.com');
INSERT INTO options (key, value) VALUES ('SMTPPort', '465');
INSERT INTO options (key, value) VALUES ('PasswordRegisterEnabled', 'true');
INSERT INTO options (key, value) VALUES ('QuotaForInvitee', '0');
INSERT INTO options (key, value) VALUES ('checkin_setting.enabled', 'false');
INSERT INTO options (key, value) VALUES ('CompletionRatio', '{
  "anthropic/claude-sonnet-4.6": 5,
  "claude-haiku-4.5": 5,
  "claude-opus-4-1-20250805-thinking": 5.25,
  "claude-opus-4-20250514-thinking": 5.25,
  "claude-opus-4-6": 5.15,
  "claude-opus-4-6-high": 5.15,
  "claude-opus-4-6-low": 5.15,
  "claude-opus-4-6-max": 5.15,
  "claude-opus-4-6-medium": 5.15,
  "claude-opus-4-6-thinking": 5.15,
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
  "gemini-3.1-pro-preview": 6,
  "gemini-3.1-pro-preview-customtools": 6,
  "gemma-4-31b-it": 5,
  "glm-5.1": 3.996960486322,
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
  "qwen/qwen3.6-plus": 6,
  "qwen3.5-35b-a3b": 1,
  "qwen3.6-plus": 3.996960486322
}');




INSERT INTO service_configs (id, merchant_pid, merchant_key, wechat_app_id, wechat_mch_id, wechat_mch_serial_no, wechat_private_key_path, wechat_apiv3_key, wechat_notify_url, alipay_app_id, alipay_private_key, alipay_public_key, alipay_notify_url, alipay_is_prod, usdt_enabled, usdt_trc20_address, usdt_trongrid_api_key, usdt_cny_rate, usdt_poll_interval_sec, usdt_expiry_minutes, created_at, updated_at, merchant_p_id, wechat_serial_no, wechat_cert_path) VALUES (1, '', '', 'xxxxx', 'xxxxx', 'xxxxx', 'xxxxx', 'xxxxx', '', '', '', '', '', false, false, '', '', 0.137, 15, 30, '2026-03-12 18:02:37.149808+08', '2026-03-12 18:23:31.82006+08', '', 'xxxxx', 'xxxxx');



INSERT INTO setups (id, version, initialized_at) VALUES (1, 'v0.11.0-alpha.6', 1771905748);


--
-- Data for Name: spatial_ref_sys; Type: TABLE DATA; Schema: public; Owner: coze
--




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






INSERT INTO two_fa_backup_codes (id, user_id, code_hash, is_used, used_at, created_at, deleted_at) VALUES (1, 10, '$2a$10$2RWTE.L9EX.DCt/.3hf7Ku6.L4ASxFQAltdElA27wfbHBfuUSstmS', false, NULL, '2026-03-10 10:16:16.246918+08', NULL);
INSERT INTO two_fa_backup_codes (id, user_id, code_hash, is_used, used_at, created_at, deleted_at) VALUES (2, 10, '$2a$10$2k5/L7k9M5E9gb/odYV7EOWlzrh1DJ22SXq7n3XrDzXa58SNo1OJK', false, NULL, '2026-03-10 10:16:16.34466+08', NULL);
INSERT INTO two_fa_backup_codes (id, user_id, code_hash, is_used, used_at, created_at, deleted_at) VALUES (3, 10, '$2a$10$gN1.ViwteOyoluTVsf1U..462fL9fAHHTDEDfyPh0UE/7KlChesSO', false, NULL, '2026-03-10 10:16:16.442146+08', NULL);
INSERT INTO two_fa_backup_codes (id, user_id, code_hash, is_used, used_at, created_at, deleted_at) VALUES (4, 10, '$2a$10$lnLnyb7Uav2FOcu3Q0ID9O6sAVmISA/LPPYHNGTHnITtce00Xjg.W', false, NULL, '2026-03-10 10:16:16.540449+08', NULL);


--
-- Data for Name: two_fas; Type: TABLE DATA; Schema: public; Owner: allrouter
--

INSERT INTO two_fas (id, user_id, secret, is_enabled, failed_attempts, locked_until, last_used_at, created_at, updated_at, deleted_at) VALUES (1, 10, 'R5T33JYMWR2ZLESDTHZ5W4XDWSPEKB2H', false, 0, NULL, NULL, '2026-03-10 10:16:15.910398+08', '2026-03-10 10:16:15.910398+08', NULL);


















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




SELECT pg_catalog.setval('paradedb._typmod_cache_id_seq', 1, false);


--
-- Name: admin_sessions_id_seq; Type: SEQUENCE SET; Schema: public; Owner: allrouter
--

SELECT pg_catalog.setval('admin_sessions_id_seq', 1, true);


--
-- Name: admin_users_id_seq; Type: SEQUENCE SET; Schema: public; Owner: allrouter
--

SELECT pg_catalog.setval('admin_users_id_seq', 1, true);


--
-- Name: channels_id_seq; Type: SEQUENCE SET; Schema: public; Owner: allrouter
--

SELECT pg_catalog.setval('channels_id_seq', 35, true);


--
-- Name: checkins_id_seq; Type: SEQUENCE SET; Schema: public; Owner: allrouter
--

SELECT pg_catalog.setval('checkins_id_seq', 2, true);


--
-- Name: custom_oauth_providers_id_seq; Type: SEQUENCE SET; Schema: public; Owner: allrouter
--

SELECT pg_catalog.setval('custom_oauth_providers_id_seq', 1, false);


--
-- Name: epay_merchants_id_seq; Type: SEQUENCE SET; Schema: public; Owner: allrouter
--

SELECT pg_catalog.setval('epay_merchants_id_seq', 1, true);


--
-- Name: invite_records_id_seq; Type: SEQUENCE SET; Schema: public; Owner: allrouter
--

SELECT pg_catalog.setval('invite_records_id_seq', 4, true);


--
-- Name: login_audit_logs_id_seq; Type: SEQUENCE SET; Schema: public; Owner: allrouter
--

SELECT pg_catalog.setval('login_audit_logs_id_seq', 1, true);


--
-- Name: logs_id_seq; Type: SEQUENCE SET; Schema: public; Owner: allrouter
--

SELECT pg_catalog.setval('logs_id_seq', 74016, true);


--
-- Name: midjourneys_id_seq; Type: SEQUENCE SET; Schema: public; Owner: allrouter
--

SELECT pg_catalog.setval('midjourneys_id_seq', 1, false);


--
-- Name: models_id_seq; Type: SEQUENCE SET; Schema: public; Owner: allrouter
--

SELECT pg_catalog.setval('models_id_seq', 17, true);


--
-- Name: orders_id_seq; Type: SEQUENCE SET; Schema: public; Owner: allrouter
--

SELECT pg_catalog.setval('orders_id_seq', 5, true);


--
-- Name: passkey_credentials_id_seq; Type: SEQUENCE SET; Schema: public; Owner: allrouter
--

SELECT pg_catalog.setval('passkey_credentials_id_seq', 1, false);


--
-- Name: payment_bill_reconcile_id_seq; Type: SEQUENCE SET; Schema: public; Owner: allrouter
--

SELECT pg_catalog.setval('payment_bill_reconcile_id_seq', 86, true);


--
-- Name: payment_bill_record_id_seq; Type: SEQUENCE SET; Schema: public; Owner: allrouter
--

SELECT pg_catalog.setval('payment_bill_record_id_seq', 86, true);


--
-- Name: prefill_groups_id_seq; Type: SEQUENCE SET; Schema: public; Owner: allrouter
--

SELECT pg_catalog.setval('prefill_groups_id_seq', 1, false);


--
-- Name: quota_data_id_seq; Type: SEQUENCE SET; Schema: public; Owner: allrouter
--

SELECT pg_catalog.setval('quota_data_id_seq', 1703, true);


--
-- Name: redemptions_id_seq; Type: SEQUENCE SET; Schema: public; Owner: allrouter
--

SELECT pg_catalog.setval('redemptions_id_seq', 11, true);


--
-- Name: service_configs_id_seq; Type: SEQUENCE SET; Schema: public; Owner: allrouter
--

SELECT pg_catalog.setval('service_configs_id_seq', 1, false);


--
-- Name: setups_id_seq; Type: SEQUENCE SET; Schema: public; Owner: allrouter
--

SELECT pg_catalog.setval('setups_id_seq', 1, true);


--
-- Name: subscription_orders_id_seq; Type: SEQUENCE SET; Schema: public; Owner: allrouter
--

SELECT pg_catalog.setval('subscription_orders_id_seq', 2, true);


--
-- Name: subscription_plans_id_seq; Type: SEQUENCE SET; Schema: public; Owner: allrouter
--

SELECT pg_catalog.setval('subscription_plans_id_seq', 1, true);


--
-- Name: subscription_pre_consume_records_id_seq; Type: SEQUENCE SET; Schema: public; Owner: allrouter
--

SELECT pg_catalog.setval('subscription_pre_consume_records_id_seq', 1, true);


--
-- Name: tasks_id_seq; Type: SEQUENCE SET; Schema: public; Owner: allrouter
--

SELECT pg_catalog.setval('tasks_id_seq', 1, false);


--
-- Name: tokens_id_seq; Type: SEQUENCE SET; Schema: public; Owner: allrouter
--

SELECT pg_catalog.setval('tokens_id_seq', 58, true);


--
-- Name: top_ups_id_seq; Type: SEQUENCE SET; Schema: public; Owner: allrouter
--

SELECT pg_catalog.setval('top_ups_id_seq', 84, true);


--
-- Name: topup_rebates_id_seq; Type: SEQUENCE SET; Schema: public; Owner: allrouter
--

SELECT pg_catalog.setval('topup_rebates_id_seq', 1, true);


--
-- Name: two_fa_backup_codes_id_seq; Type: SEQUENCE SET; Schema: public; Owner: allrouter
--

SELECT pg_catalog.setval('two_fa_backup_codes_id_seq', 4, true);


--
-- Name: two_fas_id_seq; Type: SEQUENCE SET; Schema: public; Owner: allrouter
--

SELECT pg_catalog.setval('two_fas_id_seq', 1, true);


--
-- Name: user_oauth_bindings_id_seq; Type: SEQUENCE SET; Schema: public; Owner: allrouter
--

SELECT pg_catalog.setval('user_oauth_bindings_id_seq', 1, false);


--
-- Name: user_subscriptions_id_seq; Type: SEQUENCE SET; Schema: public; Owner: allrouter
--

SELECT pg_catalog.setval('user_subscriptions_id_seq', 1, true);


--
-- Name: users_id_seq; Type: SEQUENCE SET; Schema: public; Owner: allrouter
--

SELECT pg_catalog.setval('users_id_seq', 5937, true);


--
-- Name: vendors_id_seq; Type: SEQUENCE SET; Schema: public; Owner: allrouter
--

SELECT pg_catalog.setval('vendors_id_seq', 14, true);


--
-- Name: topology_id_seq; Type: SEQUENCE SET; Schema: topology; Owner: coze
--

SELECT pg_catalog.setval('topology.topology_id_seq', 1, false);


--
-- Name: abilities abilities_pkey; Type: CONSTRAINT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY abilities
    ADD CONSTRAINT abilities_pkey PRIMARY KEY ("group", model, channel_id);


--
-- Name: admin_sessions admin_sessions_pkey; Type: CONSTRAINT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY admin_sessions
    ADD CONSTRAINT admin_sessions_pkey PRIMARY KEY (id);


--
-- Name: admin_users admin_users_pkey; Type: CONSTRAINT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY admin_users
    ADD CONSTRAINT admin_users_pkey PRIMARY KEY (id);


--
-- Name: channels channels_pkey; Type: CONSTRAINT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY channels
    ADD CONSTRAINT channels_pkey PRIMARY KEY (id);


--
-- Name: checkins checkins_pkey; Type: CONSTRAINT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY checkins
    ADD CONSTRAINT checkins_pkey PRIMARY KEY (id);


--
-- Name: cli_oauth cli_oauth_pkey; Type: CONSTRAINT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY cli_oauth
    ADD CONSTRAINT cli_oauth_pkey PRIMARY KEY (id);


--
-- Name: cli_user_oauth cli_user_oauth_pkey; Type: CONSTRAINT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY cli_user_oauth
    ADD CONSTRAINT cli_user_oauth_pkey PRIMARY KEY (id);


--
-- Name: cli_user cli_user_pkey; Type: CONSTRAINT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY cli_user
    ADD CONSTRAINT cli_user_pkey PRIMARY KEY (id);


--
-- Name: currency_stripe_config currency_stripe_config_pkey; Type: CONSTRAINT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY currency_stripe_config
    ADD CONSTRAINT currency_stripe_config_pkey PRIMARY KEY (currency);


--
-- Name: custom_oauth_providers custom_oauth_providers_pkey; Type: CONSTRAINT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY custom_oauth_providers
    ADD CONSTRAINT custom_oauth_providers_pkey PRIMARY KEY (id);


--
-- Name: epay_merchants epay_merchants_pkey; Type: CONSTRAINT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY epay_merchants
    ADD CONSTRAINT epay_merchants_pkey PRIMARY KEY (id);


--
-- Name: cli_user idx_cli_user_user_id; Type: CONSTRAINT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY cli_user
    ADD CONSTRAINT idx_cli_user_user_id UNIQUE (user_id);


--
-- Name: prefill_groups idx_prefill_groups_name; Type: CONSTRAINT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY prefill_groups
    ADD CONSTRAINT idx_prefill_groups_name UNIQUE (name);


--
-- Name: invite_records invite_records_invitee_id_key; Type: CONSTRAINT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY invite_records
    ADD CONSTRAINT invite_records_invitee_id_key UNIQUE (invitee_id);


--
-- Name: invite_records invite_records_pkey; Type: CONSTRAINT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY invite_records
    ADD CONSTRAINT invite_records_pkey PRIMARY KEY (id);


--
-- Name: login_audit_logs login_audit_logs_pkey; Type: CONSTRAINT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY login_audit_logs
    ADD CONSTRAINT login_audit_logs_pkey PRIMARY KEY (id);


--
-- Name: logs logs_pkey; Type: CONSTRAINT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY logs
    ADD CONSTRAINT logs_pkey PRIMARY KEY (id);


--
-- Name: midjourneys midjourneys_pkey; Type: CONSTRAINT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY midjourneys
    ADD CONSTRAINT midjourneys_pkey PRIMARY KEY (id);


--
-- Name: models models_pkey; Type: CONSTRAINT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY models
    ADD CONSTRAINT models_pkey PRIMARY KEY (id);


--
-- Name: options options_pkey; Type: CONSTRAINT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY options
    ADD CONSTRAINT options_pkey PRIMARY KEY (key);


--
-- Name: orders orders_pkey; Type: CONSTRAINT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY orders
    ADD CONSTRAINT orders_pkey PRIMARY KEY (id);


--
-- Name: passkey_credentials passkey_credentials_pkey; Type: CONSTRAINT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY passkey_credentials
    ADD CONSTRAINT passkey_credentials_pkey PRIMARY KEY (id);


--
-- Name: payment_bill_reconcile payment_bill_reconcile_pkey; Type: CONSTRAINT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY payment_bill_reconcile
    ADD CONSTRAINT payment_bill_reconcile_pkey PRIMARY KEY (id);


--
-- Name: payment_bill_record payment_bill_record_pkey; Type: CONSTRAINT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY payment_bill_record
    ADD CONSTRAINT payment_bill_record_pkey PRIMARY KEY (id);


--
-- Name: prefill_groups prefill_groups_pkey; Type: CONSTRAINT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY prefill_groups
    ADD CONSTRAINT prefill_groups_pkey PRIMARY KEY (id);


--
-- Name: quota_data quota_data_pkey; Type: CONSTRAINT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY quota_data
    ADD CONSTRAINT quota_data_pkey PRIMARY KEY (id);


--
-- Name: redemptions redemptions_pkey; Type: CONSTRAINT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY redemptions
    ADD CONSTRAINT redemptions_pkey PRIMARY KEY (id);


--
-- Name: service_configs service_configs_pkey; Type: CONSTRAINT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY service_configs
    ADD CONSTRAINT service_configs_pkey PRIMARY KEY (id);


--
-- Name: setups setups_pkey; Type: CONSTRAINT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY setups
    ADD CONSTRAINT setups_pkey PRIMARY KEY (id);


--
-- Name: subscription_orders subscription_orders_pkey; Type: CONSTRAINT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY subscription_orders
    ADD CONSTRAINT subscription_orders_pkey PRIMARY KEY (id);


--
-- Name: subscription_orders subscription_orders_trade_no_key; Type: CONSTRAINT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY subscription_orders
    ADD CONSTRAINT subscription_orders_trade_no_key UNIQUE (trade_no);


--
-- Name: subscription_plans subscription_plans_pkey; Type: CONSTRAINT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY subscription_plans
    ADD CONSTRAINT subscription_plans_pkey PRIMARY KEY (id);


--
-- Name: subscription_pre_consume_records subscription_pre_consume_records_pkey; Type: CONSTRAINT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY subscription_pre_consume_records
    ADD CONSTRAINT subscription_pre_consume_records_pkey PRIMARY KEY (id);


--
-- Name: tasks tasks_pkey; Type: CONSTRAINT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY tasks
    ADD CONSTRAINT tasks_pkey PRIMARY KEY (id);


--
-- Name: timezone_currency_map timezone_currency_map_pkey; Type: CONSTRAINT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY timezone_currency_map
    ADD CONSTRAINT timezone_currency_map_pkey PRIMARY KEY (timezone);


--
-- Name: tokens tokens_pkey; Type: CONSTRAINT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY tokens
    ADD CONSTRAINT tokens_pkey PRIMARY KEY (id);


--
-- Name: top_ups top_ups_pkey; Type: CONSTRAINT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY top_ups
    ADD CONSTRAINT top_ups_pkey PRIMARY KEY (id);


--
-- Name: top_ups top_ups_trade_no_key; Type: CONSTRAINT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY top_ups
    ADD CONSTRAINT top_ups_trade_no_key UNIQUE (trade_no);


--
-- Name: topup_rebates topup_rebates_pkey; Type: CONSTRAINT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY topup_rebates
    ADD CONSTRAINT topup_rebates_pkey PRIMARY KEY (id);


--
-- Name: two_fa_backup_codes two_fa_backup_codes_pkey; Type: CONSTRAINT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY two_fa_backup_codes
    ADD CONSTRAINT two_fa_backup_codes_pkey PRIMARY KEY (id);


--
-- Name: two_fas two_fas_pkey; Type: CONSTRAINT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY two_fas
    ADD CONSTRAINT two_fas_pkey PRIMARY KEY (id);


--
-- Name: two_fas two_fas_user_id_key; Type: CONSTRAINT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY two_fas
    ADD CONSTRAINT two_fas_user_id_key UNIQUE (user_id);


--
-- Name: payment_bill_reconcile uk_payment_bill_reconcile_channel_record; Type: CONSTRAINT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY payment_bill_reconcile
    ADD CONSTRAINT uk_payment_bill_reconcile_channel_record UNIQUE (channel_type, bill_record_id);


--
-- Name: payment_bill_record uk_payment_bill_record_channel_row_hash; Type: CONSTRAINT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY payment_bill_record
    ADD CONSTRAINT uk_payment_bill_record_channel_row_hash UNIQUE (channel_type, row_hash);


--
-- Name: user_oauth_bindings user_oauth_bindings_pkey; Type: CONSTRAINT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY user_oauth_bindings
    ADD CONSTRAINT user_oauth_bindings_pkey PRIMARY KEY (id);


--
-- Name: user_subscriptions user_subscriptions_pkey; Type: CONSTRAINT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY user_subscriptions
    ADD CONSTRAINT user_subscriptions_pkey PRIMARY KEY (id);


--
-- Name: users users_pkey; Type: CONSTRAINT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);


--
-- Name: users users_username_key; Type: CONSTRAINT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY users
    ADD CONSTRAINT users_username_key UNIQUE (username);


--
-- Name: vendors vendors_pkey; Type: CONSTRAINT; Schema: public; Owner: allrouter
--

ALTER TABLE ONLY vendors
    ADD CONSTRAINT vendors_pkey PRIMARY KEY (id);


--
-- Name: idx_abilities_channel_id; Type: INDEX; Schema: public; Owner: allrouter
--

CREATE INDEX idx_abilities_channel_id ON abilities USING btree (channel_id);


--
-- Name: idx_abilities_priority; Type: INDEX; Schema: public; Owner: allrouter
--

CREATE INDEX idx_abilities_priority ON abilities USING btree (priority);


--
-- Name: idx_abilities_tag; Type: INDEX; Schema: public; Owner: allrouter
--

CREATE INDEX idx_abilities_tag ON abilities USING btree (tag);


--
-- Name: idx_abilities_weight; Type: INDEX; Schema: public; Owner: allrouter
--

CREATE INDEX idx_abilities_weight ON abilities USING btree (weight);


--
-- Name: idx_admin_sessions_expires_at; Type: INDEX; Schema: public; Owner: allrouter
--

CREATE INDEX idx_admin_sessions_expires_at ON admin_sessions USING btree (expires_at);


--
-- Name: idx_admin_sessions_token; Type: INDEX; Schema: public; Owner: allrouter
--

CREATE UNIQUE INDEX idx_admin_sessions_token ON admin_sessions USING btree (token);


--
-- Name: idx_admin_sessions_user_id; Type: INDEX; Schema: public; Owner: allrouter
--

CREATE INDEX idx_admin_sessions_user_id ON admin_sessions USING btree (user_id);


--
-- Name: idx_admin_users_username; Type: INDEX; Schema: public; Owner: allrouter
--

CREATE UNIQUE INDEX idx_admin_users_username ON admin_users USING btree (username);


--
-- Name: idx_channels_name; Type: INDEX; Schema: public; Owner: allrouter
--

CREATE INDEX idx_channels_name ON channels USING btree (name);


--
-- Name: idx_channels_tag; Type: INDEX; Schema: public; Owner: allrouter
--

CREATE INDEX idx_channels_tag ON channels USING btree (tag);


--
-- Name: idx_created_at_id; Type: INDEX; Schema: public; Owner: allrouter
--

CREATE INDEX idx_created_at_id ON logs USING btree (id, created_at);


--
-- Name: idx_created_at_type; Type: INDEX; Schema: public; Owner: allrouter
--

CREATE INDEX idx_created_at_type ON logs USING btree (created_at, type);


--
-- Name: idx_custom_oauth_providers_slug; Type: INDEX; Schema: public; Owner: allrouter
--

CREATE UNIQUE INDEX idx_custom_oauth_providers_slug ON custom_oauth_providers USING btree (slug);


--
-- Name: idx_epay_merchants_p_id; Type: INDEX; Schema: public; Owner: allrouter
--

CREATE UNIQUE INDEX idx_epay_merchants_p_id ON epay_merchants USING btree (pid);


--
-- Name: idx_epay_merchants_pid; Type: INDEX; Schema: public; Owner: allrouter
--

CREATE UNIQUE INDEX idx_epay_merchants_pid ON epay_merchants USING btree (pid);


--
-- Name: idx_invite_records_created_at; Type: INDEX; Schema: public; Owner: allrouter
--

CREATE INDEX idx_invite_records_created_at ON invite_records USING btree (created_at);


--
-- Name: idx_invite_records_invitee_id; Type: INDEX; Schema: public; Owner: allrouter
--

CREATE UNIQUE INDEX idx_invite_records_invitee_id ON invite_records USING btree (invitee_id);


--
-- Name: idx_invite_records_inviter_id; Type: INDEX; Schema: public; Owner: allrouter
--

CREATE INDEX idx_invite_records_inviter_id ON invite_records USING btree (inviter_id);


--
-- Name: idx_invite_records_register_time; Type: INDEX; Schema: public; Owner: allrouter
--

CREATE INDEX idx_invite_records_register_time ON invite_records USING btree (register_time);


--
-- Name: idx_login_audit_logs_created_at; Type: INDEX; Schema: public; Owner: allrouter
--

CREATE INDEX idx_login_audit_logs_created_at ON login_audit_logs USING btree (created_at);


--
-- Name: idx_login_audit_logs_username; Type: INDEX; Schema: public; Owner: allrouter
--

CREATE INDEX idx_login_audit_logs_username ON login_audit_logs USING btree (username);


--
-- Name: idx_logs_channel_id; Type: INDEX; Schema: public; Owner: allrouter
--

CREATE INDEX idx_logs_channel_id ON logs USING btree (channel_id);


--
-- Name: idx_logs_group; Type: INDEX; Schema: public; Owner: allrouter
--

CREATE INDEX idx_logs_group ON logs USING btree ("group");


--
-- Name: idx_logs_ip; Type: INDEX; Schema: public; Owner: allrouter
--

CREATE INDEX idx_logs_ip ON logs USING btree (ip);


--
-- Name: idx_logs_model_name; Type: INDEX; Schema: public; Owner: allrouter
--

CREATE INDEX idx_logs_model_name ON logs USING btree (model_name);


--
-- Name: idx_logs_request_id; Type: INDEX; Schema: public; Owner: allrouter
--

CREATE INDEX idx_logs_request_id ON logs USING btree (request_id);


--
-- Name: idx_logs_token_id; Type: INDEX; Schema: public; Owner: allrouter
--

CREATE INDEX idx_logs_token_id ON logs USING btree (token_id);


--
-- Name: idx_logs_token_name; Type: INDEX; Schema: public; Owner: allrouter
--

CREATE INDEX idx_logs_token_name ON logs USING btree (token_name);


--
-- Name: idx_logs_user_id; Type: INDEX; Schema: public; Owner: allrouter
--

CREATE INDEX idx_logs_user_id ON logs USING btree (user_id);


--
-- Name: idx_logs_username; Type: INDEX; Schema: public; Owner: allrouter
--

CREATE INDEX idx_logs_username ON logs USING btree (username);


--
-- Name: idx_midjourneys_action; Type: INDEX; Schema: public; Owner: allrouter
--

CREATE INDEX idx_midjourneys_action ON midjourneys USING btree (action);


--
-- Name: idx_midjourneys_finish_time; Type: INDEX; Schema: public; Owner: allrouter
--

CREATE INDEX idx_midjourneys_finish_time ON midjourneys USING btree (finish_time);


--
-- Name: idx_midjourneys_mj_id; Type: INDEX; Schema: public; Owner: allrouter
--

CREATE INDEX idx_midjourneys_mj_id ON midjourneys USING btree (mj_id);


--
-- Name: idx_midjourneys_progress; Type: INDEX; Schema: public; Owner: allrouter
--

CREATE INDEX idx_midjourneys_progress ON midjourneys USING btree (progress);


--
-- Name: idx_midjourneys_start_time; Type: INDEX; Schema: public; Owner: allrouter
--

CREATE INDEX idx_midjourneys_start_time ON midjourneys USING btree (start_time);


--
-- Name: idx_midjourneys_status; Type: INDEX; Schema: public; Owner: allrouter
--

CREATE INDEX idx_midjourneys_status ON midjourneys USING btree (status);


--
-- Name: idx_midjourneys_submit_time; Type: INDEX; Schema: public; Owner: allrouter
--

CREATE INDEX idx_midjourneys_submit_time ON midjourneys USING btree (submit_time);


--
-- Name: idx_midjourneys_user_id; Type: INDEX; Schema: public; Owner: allrouter
--

CREATE INDEX idx_midjourneys_user_id ON midjourneys USING btree (user_id);


--
-- Name: idx_models_deleted_at; Type: INDEX; Schema: public; Owner: allrouter
--

CREATE INDEX idx_models_deleted_at ON models USING btree (deleted_at);


--
-- Name: idx_models_vendor_id; Type: INDEX; Schema: public; Owner: allrouter
--

CREATE INDEX idx_models_vendor_id ON models USING btree (vendor_id);


--
-- Name: idx_orders_callback_status; Type: INDEX; Schema: public; Owner: allrouter
--

CREATE INDEX idx_orders_callback_status ON orders USING btree (callback_status);


--
-- Name: idx_orders_out_trade_no; Type: INDEX; Schema: public; Owner: allrouter
--

CREATE UNIQUE INDEX idx_orders_out_trade_no ON orders USING btree (out_trade_no);


--
-- Name: idx_orders_status; Type: INDEX; Schema: public; Owner: allrouter
--

CREATE INDEX idx_orders_status ON orders USING btree (status);


--
-- Name: idx_passkey_credentials_credential_id; Type: INDEX; Schema: public; Owner: allrouter
--

CREATE UNIQUE INDEX idx_passkey_credentials_credential_id ON passkey_credentials USING btree (credential_id);


--
-- Name: idx_passkey_credentials_deleted_at; Type: INDEX; Schema: public; Owner: allrouter
--

CREATE INDEX idx_passkey_credentials_deleted_at ON passkey_credentials USING btree (deleted_at);


--
-- Name: idx_passkey_credentials_user_id; Type: INDEX; Schema: public; Owner: allrouter
--

CREATE UNIQUE INDEX idx_passkey_credentials_user_id ON passkey_credentials USING btree (user_id);


--
-- Name: idx_payment_bill_reconcile_bill_date; Type: INDEX; Schema: public; Owner: allrouter
--

CREATE INDEX idx_payment_bill_reconcile_bill_date ON payment_bill_reconcile USING btree (bill_date);


--
-- Name: idx_payment_bill_reconcile_bill_record_id; Type: INDEX; Schema: public; Owner: allrouter
--

CREATE INDEX idx_payment_bill_reconcile_bill_record_id ON payment_bill_reconcile USING btree (bill_record_id);


--
-- Name: idx_payment_bill_reconcile_channel_key; Type: INDEX; Schema: public; Owner: allrouter
--

CREATE UNIQUE INDEX idx_payment_bill_reconcile_channel_key ON payment_bill_reconcile USING btree (channel_type, reconcile_key);


--
-- Name: idx_payment_bill_reconcile_channel_status; Type: INDEX; Schema: public; Owner: allrouter
--

CREATE INDEX idx_payment_bill_reconcile_channel_status ON payment_bill_reconcile USING btree (channel_status);


--
-- Name: idx_payment_bill_reconcile_channel_trade_no; Type: INDEX; Schema: public; Owner: allrouter
--

CREATE INDEX idx_payment_bill_reconcile_channel_trade_no ON payment_bill_reconcile USING btree (channel_trade_no);


--
-- Name: idx_payment_bill_reconcile_channel_type; Type: INDEX; Schema: public; Owner: allrouter
--

CREATE INDEX idx_payment_bill_reconcile_channel_type ON payment_bill_reconcile USING btree (channel_type);


--
-- Name: idx_payment_bill_reconcile_local_complete_time; Type: INDEX; Schema: public; Owner: allrouter
--

CREATE INDEX idx_payment_bill_reconcile_local_complete_time ON payment_bill_reconcile USING btree (local_complete_time);


--
-- Name: idx_payment_bill_reconcile_local_create_time; Type: INDEX; Schema: public; Owner: allrouter
--

CREATE INDEX idx_payment_bill_reconcile_local_create_time ON payment_bill_reconcile USING btree (local_create_time);


--
-- Name: idx_payment_bill_reconcile_local_id; Type: INDEX; Schema: public; Owner: allrouter
--

CREATE INDEX idx_payment_bill_reconcile_local_id ON payment_bill_reconcile USING btree (local_id);


--
-- Name: idx_payment_bill_reconcile_local_status; Type: INDEX; Schema: public; Owner: allrouter
--

CREATE INDEX idx_payment_bill_reconcile_local_status ON payment_bill_reconcile USING btree (local_status);


--
-- Name: idx_payment_bill_reconcile_local_trade_no; Type: INDEX; Schema: public; Owner: allrouter
--

CREATE INDEX idx_payment_bill_reconcile_local_trade_no ON payment_bill_reconcile USING btree (local_trade_no);


--
-- Name: idx_payment_bill_reconcile_local_type; Type: INDEX; Schema: public; Owner: allrouter
--

CREATE INDEX idx_payment_bill_reconcile_local_type ON payment_bill_reconcile USING btree (local_type);


--
-- Name: idx_payment_bill_reconcile_merchant_trade_no; Type: INDEX; Schema: public; Owner: allrouter
--

CREATE INDEX idx_payment_bill_reconcile_merchant_trade_no ON payment_bill_reconcile USING btree (merchant_trade_no);


--
-- Name: idx_payment_bill_reconcile_reconcile_reason; Type: INDEX; Schema: public; Owner: allrouter
--

CREATE INDEX idx_payment_bill_reconcile_reconcile_reason ON payment_bill_reconcile USING btree (reconcile_reason);


--
-- Name: idx_payment_bill_reconcile_reconcile_status; Type: INDEX; Schema: public; Owner: allrouter
--

CREATE INDEX idx_payment_bill_reconcile_reconcile_status ON payment_bill_reconcile USING btree (reconcile_status);


--
-- Name: idx_payment_bill_reconcile_record_source; Type: INDEX; Schema: public; Owner: allrouter
--

CREATE INDEX idx_payment_bill_reconcile_record_source ON payment_bill_reconcile USING btree (record_source);


--
-- Name: idx_payment_bill_reconcile_trade_time; Type: INDEX; Schema: public; Owner: allrouter
--

CREATE INDEX idx_payment_bill_reconcile_trade_time ON payment_bill_reconcile USING btree (trade_time);


--
-- Name: idx_payment_bill_record_bill_date; Type: INDEX; Schema: public; Owner: allrouter
--

CREATE INDEX idx_payment_bill_record_bill_date ON payment_bill_record USING btree (bill_date);


CREATE INDEX idx_payment_bill_record_channel_refund_no ON payment_bill_record USING btree (channel_refund_no);


CREATE UNIQUE INDEX idx_payment_bill_record_channel_row_hash ON payment_bill_record USING btree (channel_type, row_hash);


CREATE INDEX idx_payment_bill_record_channel_trade_no ON payment_bill_record USING btree (channel_trade_no);

CREATE INDEX idx_payment_bill_record_channel_type ON payment_bill_record USING btree (channel_type);

CREATE INDEX idx_payment_bill_record_mch_id ON payment_bill_record USING btree (mch_id);

CREATE INDEX idx_payment_bill_record_merchant_refund_no ON payment_bill_record USING btree (merchant_refund_no);

CREATE INDEX idx_payment_bill_record_merchant_trade_no ON payment_bill_record USING btree (merchant_trade_no);

CREATE INDEX idx_payment_bill_record_row_index ON payment_bill_record USING btree (row_index);

CREATE INDEX idx_payment_bill_record_trade_status ON payment_bill_record USING btree (trade_status);

CREATE INDEX idx_payment_bill_record_trade_time ON payment_bill_record USING btree (trade_time);

CREATE INDEX idx_prefill_groups_deleted_at ON prefill_groups USING btree (deleted_at);

CREATE INDEX idx_prefill_groups_type ON prefill_groups USING btree (type);

CREATE INDEX idx_qdt_created_at ON quota_data USING btree (created_at);

CREATE INDEX idx_qdt_model_user_name ON quota_data USING btree (model_name, username);

CREATE INDEX idx_quota_data_user_id ON quota_data USING btree (user_id);

CREATE INDEX idx_redemptions_deleted_at ON redemptions USING btree (deleted_at);

CREATE UNIQUE INDEX idx_redemptions_key ON redemptions USING btree (key);

CREATE INDEX idx_redemptions_name ON redemptions USING btree (name);

CREATE INDEX idx_subscription_orders_plan_id ON subscription_orders USING btree (plan_id);

CREATE INDEX idx_subscription_orders_trade_no ON subscription_orders USING btree (trade_no);

CREATE INDEX idx_subscription_orders_user_id ON subscription_orders USING btree (user_id);

CREATE UNIQUE INDEX idx_subscription_pre_consume_records_request_id ON subscription_pre_consume_records USING btree (request_id);

CREATE INDEX idx_subscription_pre_consume_records_status ON subscription_pre_consume_records USING btree (status);

CREATE INDEX idx_subscription_pre_consume_records_updated_at ON subscription_pre_consume_records USING btree (updated_at);

CREATE INDEX idx_subscription_pre_consume_records_user_id ON subscription_pre_consume_records USING btree (user_id);

CREATE INDEX idx_subscription_pre_consume_records_user_subscription_id ON subscription_pre_consume_records USING btree (user_subscription_id);

CREATE INDEX idx_tasks_action ON tasks USING btree (action);

CREATE INDEX idx_tasks_channel_id ON tasks USING btree (channel_id);

CREATE INDEX idx_tasks_created_at ON tasks USING btree (created_at);

CREATE INDEX idx_tasks_finish_time ON tasks USING btree (finish_time);

CREATE INDEX idx_tasks_platform ON tasks USING btree (platform);

CREATE INDEX idx_tasks_progress ON tasks USING btree (progress);

CREATE INDEX idx_tasks_start_time ON tasks USING btree (start_time);

CREATE INDEX idx_tasks_status ON tasks USING btree (status);

CREATE INDEX idx_tasks_submit_time ON tasks USING btree (submit_time);

CREATE INDEX idx_tasks_task_id ON tasks USING btree (task_id);

CREATE INDEX idx_tasks_user_id ON tasks USING btree (user_id);

CREATE INDEX idx_tokens_deleted_at ON tokens USING btree (deleted_at);

CREATE UNIQUE INDEX idx_tokens_key ON tokens USING btree (key);

CREATE INDEX idx_tokens_name ON tokens USING btree (name);

CREATE INDEX idx_tokens_user_id ON tokens USING btree (user_id);

CREATE INDEX idx_top_ups_biz_type ON top_ups USING btree (biz_type);

CREATE INDEX idx_top_ups_source_id ON top_ups USING btree (source_id);

CREATE INDEX idx_top_ups_trade_no ON top_ups USING btree (trade_no);

CREATE INDEX idx_top_ups_user_id ON top_ups USING btree (user_id);

CREATE INDEX idx_topup_rebates_created_at ON topup_rebates USING btree (created_at);

CREATE INDEX idx_topup_rebates_invitee_id ON topup_rebates USING btree (invitee_id);

CREATE INDEX idx_topup_rebates_inviter_id ON topup_rebates USING btree (inviter_id);

CREATE UNIQUE INDEX idx_topup_rebates_top_up_id ON topup_rebates USING btree (topup_id);

CREATE UNIQUE INDEX idx_topup_rebates_topup_id ON topup_rebates USING btree (topup_id);

CREATE INDEX idx_topup_rebates_trade_no ON topup_rebates USING btree (trade_no);

CREATE INDEX idx_two_fa_backup_codes_deleted_at ON two_fa_backup_codes USING btree (deleted_at);

CREATE INDEX idx_two_fa_backup_codes_user_id ON two_fa_backup_codes USING btree (user_id);

CREATE INDEX idx_two_fas_deleted_at ON two_fas USING btree (deleted_at);

CREATE INDEX idx_two_fas_user_id ON two_fas USING btree (user_id);

CREATE UNIQUE INDEX idx_user_checkin_date ON checkins USING btree (user_id, checkin_date);

CREATE UNIQUE INDEX idx_user_id ON cli_user USING btree (user_id);

CREATE INDEX idx_user_id_id ON logs USING btree (user_id, id);

CREATE INDEX idx_user_sub_active ON user_subscriptions USING btree (user_id, status, end_time);

CREATE INDEX idx_user_subscriptions_end_time ON user_subscriptions USING btree (end_time);

CREATE INDEX idx_user_subscriptions_next_reset_time ON user_subscriptions USING btree (next_reset_time);

CREATE INDEX idx_user_subscriptions_plan_id ON user_subscriptions USING btree (plan_id);

CREATE INDEX idx_user_subscriptions_status ON user_subscriptions USING btree (status);

CREATE INDEX idx_user_subscriptions_user_id ON user_subscriptions USING btree (user_id);

CREATE UNIQUE INDEX idx_users_access_token ON users USING btree (access_token);

CREATE UNIQUE INDEX idx_users_aff_code ON users USING btree (aff_code);

CREATE INDEX idx_users_deleted_at ON users USING btree (deleted_at);

CREATE INDEX idx_users_discord_id ON users USING btree (discord_id);

CREATE INDEX idx_users_display_name ON users USING btree (display_name);

CREATE INDEX idx_users_email ON users USING btree (email);

CREATE INDEX idx_users_git_hub_id ON users USING btree (github_id);

CREATE INDEX idx_users_inviter_id ON users USING btree (inviter_id);

CREATE INDEX idx_users_linux_do_id ON users USING btree (linux_do_id);

CREATE INDEX idx_users_oidc_id ON users USING btree (oidc_id);

CREATE INDEX idx_users_stripe_customer ON users USING btree (stripe_customer);

CREATE INDEX idx_users_telegram_id ON users USING btree (telegram_id);

CREATE INDEX idx_users_username ON users USING btree (username);

CREATE INDEX idx_users_we_chat_id ON users USING btree (wechat_id);

CREATE INDEX idx_vendors_deleted_at ON vendors USING btree (deleted_at);

CREATE INDEX index_username_model_name ON logs USING btree (model_name, username);

CREATE UNIQUE INDEX uk_model_name_delete_at ON models USING btree (model_name, deleted_at);

CREATE UNIQUE INDEX uk_prefill_name ON prefill_groups USING btree (name) WHERE (deleted_at IS NULL);

CREATE UNIQUE INDEX uk_vendor_name_delete_at ON vendors USING btree (name, deleted_at);

CREATE UNIQUE INDEX ux_provider_userid ON user_oauth_bindings USING btree (provider_id, provider_user_id);

CREATE UNIQUE INDEX ux_user_provider ON user_oauth_bindings USING btree (user_id, provider_id);

ALTER TABLE ONLY timezone_currency_map
    ADD CONSTRAINT timezone_currency_map_currency_fkey FOREIGN KEY (currency) REFERENCES currency_stripe_config(currency);


-- PostgreSQL 初始化脚本：奖励所得额度 + 两级消费返利。
-- 说明：一级消费返利还没有上线时，直接执行本脚本即可；返利比例默认写入 0，后台配置后才会生效。

-- 1. 为用户表新增奖励所得额度字段。此字段只记录系统奖励/返利所得额度的剩余额度。
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS reward_quota integer NOT NULL DEFAULT 0;

COMMENT ON COLUMN users.reward_quota IS '奖励所得额度：注册奖励、邀请奖励、签到、兑换、消费返利等系统奖励累计剩余额度';

UPDATE users
SET reward_quota = 0
WHERE reward_quota IS NULL;

ALTER TABLE users
    ALTER COLUMN reward_quota SET DEFAULT 0,
    ALTER COLUMN reward_quota SET NOT NULL;

-- 2. 新增消费返利记录表。
CREATE TABLE IF NOT EXISTS consume_rebates (
    id integer GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
    inviter_id integer NOT NULL DEFAULT 0,
    invitee_id integer NOT NULL DEFAULT 0,
    request_id varchar(64) NOT NULL DEFAULT '',
    level integer NOT NULL DEFAULT 1,
    source_quota integer NOT NULL DEFAULT 0,
    rebate_ratio double precision NOT NULL DEFAULT 0,
    rebate_quota integer NOT NULL DEFAULT 0,
    created_at bigint NOT NULL DEFAULT 0
);

COMMENT ON TABLE consume_rebates IS '消费返利记录表：被邀请人使用充值额度消费后，给上级邀请人生成的返利记录';
COMMENT ON COLUMN consume_rebates.id IS '主键ID';
COMMENT ON COLUMN consume_rebates.inviter_id IS '获得返利的邀请人用户ID';
COMMENT ON COLUMN consume_rebates.invitee_id IS '产生消费的被邀请人用户ID';
COMMENT ON COLUMN consume_rebates.request_id IS '消费请求ID：用于同一次消费返利幂等去重';
COMMENT ON COLUMN consume_rebates.level IS '返利层级：1表示一级消费返利，2表示二级消费返利';
COMMENT ON COLUMN consume_rebates.source_quota IS '参与返利计算的原始消费额度，只统计充值额度消费，不统计订阅额度和奖励额度';
COMMENT ON COLUMN consume_rebates.rebate_ratio IS '返利比例，单位为百分比';
COMMENT ON COLUMN consume_rebates.rebate_quota IS '本次实际返利额度';
COMMENT ON COLUMN consume_rebates.created_at IS '创建时间，Unix时间戳秒';

-- 3. 幂等补齐 level 字段。首次上线时不会产生额外影响，只是方便重复执行脚本。
ALTER TABLE consume_rebates
    ADD COLUMN IF NOT EXISTS level integer NOT NULL DEFAULT 1;

UPDATE consume_rebates
SET level = 1
WHERE level IS NULL;

ALTER TABLE consume_rebates
    ALTER COLUMN level SET DEFAULT 1,
    ALTER COLUMN level SET NOT NULL;

COMMENT ON COLUMN consume_rebates.level IS '返利层级：1表示一级消费返利，2表示二级消费返利';

-- 4. 新增索引。同一次接口请求最多生成一条一级返利和一条二级返利。
DROP INDEX IF EXISTS idx_consume_rebates_request_id;
DROP INDEX IF EXISTS idx_consume_rebate_request_id;

CREATE INDEX IF NOT EXISTS idx_consume_rebates_inviter_id
    ON consume_rebates(inviter_id);

CREATE INDEX IF NOT EXISTS idx_consume_rebates_invitee_id
    ON consume_rebates(invitee_id);

CREATE INDEX IF NOT EXISTS idx_consume_rebates_created_at
    ON consume_rebates(created_at);

CREATE UNIQUE INDEX IF NOT EXISTS idx_consume_rebate_request_level
    ON consume_rebates(request_id, level);

-- 5. 新增消费返利比例配置。
-- InviteTopupRebateRatio：一级消费返利比例，单位百分比，0 表示关闭。
-- InviteConsumeRebateRatioLevel2：二级消费返利比例，单位百分比，0 表示关闭。

INSERT INTO options ("key", "value")
VALUES ('InviteConsumeRebateRatioLevel2', '0')
ON CONFLICT ("key") DO NOTHING;


CREATE TABLE IF NOT EXISTS provider_profits (
                                                id BIGSERIAL PRIMARY KEY,
                                                provider_id INTEGER NOT NULL,
                                                owner_user_id INTEGER NOT NULL,
                                                provider_user_id INTEGER NOT NULL,
                                                request_id VARCHAR(64) NOT NULL UNIQUE,
    public_model_name VARCHAR(255),
    base_model_name VARCHAR(255),
    provider_user_quota INTEGER NOT NULL DEFAULT 0,
    base_cost_quota INTEGER NOT NULL DEFAULT 0,
    paid_quota INTEGER NOT NULL DEFAULT 0,
    covered_cost_quota INTEGER NOT NULL DEFAULT 0,
    owner_cost_quota INTEGER NOT NULL DEFAULT 0,
    profit_quota INTEGER NOT NULL DEFAULT 0,
    profit_settled BOOLEAN NOT NULL DEFAULT FALSE,
    owner_cost_settled BOOLEAN NOT NULL DEFAULT FALSE,
    created_at BIGINT
    );

CREATE INDEX IF NOT EXISTS idx_provider_profits_provider_id ON provider_profits(provider_id);
CREATE INDEX IF NOT EXISTS idx_provider_profits_owner_user_id ON provider_profits(owner_user_id);
CREATE INDEX IF NOT EXISTS idx_provider_profits_provider_user_id ON provider_profits(provider_user_id);
CREATE INDEX IF NOT EXISTS idx_provider_profits_created_at ON provider_profits(created_at);
CREATE INDEX IF NOT EXISTS idx_provider_profits_profit_settled ON provider_profits(profit_settled);
CREATE INDEX IF NOT EXISTS idx_provider_profits_owner_cost_settled ON provider_profits(owner_cost_settled);

COMMENT ON TABLE provider_profits IS '服务商成功消费后的即时成本和利润入账记录';
COMMENT ON COLUMN provider_profits.id IS '主键ID';
COMMENT ON COLUMN provider_profits.provider_id IS '服务商ID';
COMMENT ON COLUMN provider_profits.owner_user_id IS '服务商主账号用户ID';
COMMENT ON COLUMN provider_profits.provider_user_id IS '服务商站点下发起调用的用户ID';
COMMENT ON COLUMN provider_profits.request_id IS '请求ID，用于保证同一次调用只结算一次';
COMMENT ON COLUMN provider_profits.public_model_name IS '服务商对外展示和售卖的模型名称';
COMMENT ON COLUMN provider_profits.base_model_name IS '实际调用主站的基础模型名称';
COMMENT ON COLUMN provider_profits.provider_user_quota IS '服务商用户本次应扣额度，按服务商定价计算';
COMMENT ON COLUMN provider_profits.base_cost_quota IS '主站原价成本额度';
COMMENT ON COLUMN provider_profits.paid_quota IS '服务商用户本次实际消耗的充值余额额度，不含奖励余额和订阅额度';
COMMENT ON COLUMN provider_profits.covered_cost_quota IS '充值余额已覆盖的主站成本额度';
COMMENT ON COLUMN provider_profits.owner_cost_quota IS '还需要服务商主账号承担的成本额度';
COMMENT ON COLUMN provider_profits.profit_quota IS '本次即时入账给服务商主账号的利润额度';
COMMENT ON COLUMN provider_profits.profit_settled IS '服务商利润是否已入账';
COMMENT ON COLUMN provider_profits.owner_cost_settled IS '服务商主账号成本是否已扣除';
COMMENT ON COLUMN provider_profits.created_at IS '创建时间戳';



-- PostgreSQL 服务商租户隔离迁移脚本。
-- 执行前请先检查脚本并备份数据库。

BEGIN;

CREATE TABLE IF NOT EXISTS providers (
                                         id BIGSERIAL PRIMARY KEY,
                                         owner_user_id BIGINT NOT NULL,
                                         name VARCHAR(128) NOT NULL,
    status INTEGER NOT NULL DEFAULT 1,
    created_at BIGINT,
    updated_at BIGINT
    );

CREATE INDEX IF NOT EXISTS idx_providers_owner_user_id ON providers(owner_user_id);
CREATE INDEX IF NOT EXISTS idx_providers_status ON providers(status);

CREATE TABLE IF NOT EXISTS provider_domains (
                                                id BIGSERIAL PRIMARY KEY,
                                                provider_id BIGINT NOT NULL,
                                                domain VARCHAR(255) NOT NULL,
    status INTEGER NOT NULL DEFAULT 0,
    verify_token VARCHAR(64),
    created_at BIGINT,
    updated_at BIGINT
    );

CREATE UNIQUE INDEX IF NOT EXISTS idx_provider_domains_domain ON provider_domains(domain);
CREATE INDEX IF NOT EXISTS idx_provider_domains_provider_id ON provider_domains(provider_id);
CREATE INDEX IF NOT EXISTS idx_provider_domains_status ON provider_domains(status);

CREATE TABLE IF NOT EXISTS provider_configs (
                                                id BIGSERIAL PRIMARY KEY,
                                                provider_id BIGINT NOT NULL,
                                                site_name VARCHAR(128),
    logo TEXT,
    theme_color VARCHAR(32),
    login_background TEXT,
    home_modules TEXT,
    nav_modules TEXT,
    pricing_display TEXT,
    announcement TEXT,
    footer_text TEXT,
    support_url TEXT,
    wechat_support VARCHAR(128),
    qq_support VARCHAR(128),
    created_at BIGINT,
    updated_at BIGINT
    );

CREATE UNIQUE INDEX IF NOT EXISTS idx_provider_configs_provider_id ON provider_configs(provider_id);

CREATE TABLE IF NOT EXISTS provider_model_pricings (
                                                       id BIGSERIAL PRIMARY KEY,
                                                       provider_id BIGINT NOT NULL,
                                                       public_model_name VARCHAR(255) NOT NULL,
    base_model_name VARCHAR(255) NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT true,
    pricing_type VARCHAR(16) NOT NULL DEFAULT 'ratio',
    ratio DECIMAL(18,8) NOT NULL DEFAULT 1,
    delta_model_ratio DECIMAL(18,8) NOT NULL DEFAULT 0,
    delta_model_price DECIMAL(18,8) NOT NULL DEFAULT 0,
    created_at BIGINT,
    updated_at BIGINT
    );

CREATE UNIQUE INDEX IF NOT EXISTS idx_provider_public_model
    ON provider_model_pricings(provider_id, public_model_name);
CREATE INDEX IF NOT EXISTS idx_provider_model_pricings_provider_id ON provider_model_pricings(provider_id);
CREATE INDEX IF NOT EXISTS idx_provider_model_pricings_base_model_name ON provider_model_pricings(base_model_name);
CREATE INDEX IF NOT EXISTS idx_provider_model_pricings_enabled ON provider_model_pricings(enabled);

ALTER TABLE users ADD COLUMN IF NOT EXISTS provider_id INTEGER NOT NULL DEFAULT 0;
ALTER TABLE tokens ADD COLUMN IF NOT EXISTS provider_id INTEGER NOT NULL DEFAULT 0;
ALTER TABLE top_ups ADD COLUMN IF NOT EXISTS provider_id INTEGER NOT NULL DEFAULT 0;
ALTER TABLE logs ADD COLUMN IF NOT EXISTS provider_id INTEGER NOT NULL DEFAULT 0;
ALTER TABLE logs ADD COLUMN IF NOT EXISTS base_model_name VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE logs ADD COLUMN IF NOT EXISTS billing_side VARCHAR(32) NOT NULL DEFAULT '';

-- 旧版本通常存在全局用户名唯一索引。
-- 服务商租户隔离后，用户名需要改为按服务商维度唯一。
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_username_key;
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_email_key;
ALTER TABLE users DROP CONSTRAINT IF EXISTS uni_users_username;
ALTER TABLE users DROP CONSTRAINT IF EXISTS uni_users_email;
DROP INDEX IF EXISTS uni_users_username;
DROP INDEX IF EXISTS uni_users_email;
DROP INDEX IF EXISTS idx_users_username;
DROP INDEX IF EXISTS idx_users_email;

CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_provider_id ON users(provider_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_provider_username
    ON users(provider_id, username);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_provider_email
    ON users(provider_id, email)
    WHERE email IS NOT NULL AND email <> '';

CREATE INDEX IF NOT EXISTS idx_tokens_provider_id ON tokens(provider_id);
CREATE INDEX IF NOT EXISTS idx_tokens_user_provider ON tokens(user_id, provider_id);
CREATE INDEX IF NOT EXISTS idx_top_ups_provider_id ON top_ups(provider_id);
CREATE INDEX IF NOT EXISTS idx_top_ups_user_provider ON top_ups(user_id, provider_id);
CREATE INDEX IF NOT EXISTS idx_logs_provider_id ON logs(provider_id);
CREATE INDEX IF NOT EXISTS idx_logs_billing_side ON logs(billing_side);
CREATE INDEX IF NOT EXISTS idx_logs_base_model_name ON logs(base_model_name);

COMMENT ON TABLE providers IS '服务商主表：每一条记录代表一个独立服务商，不支持下级服务商';
COMMENT ON COLUMN providers.id IS '服务商 ID，主键';
COMMENT ON COLUMN providers.owner_user_id IS '服务商归属的主站用户 ID；服务商调用主站模型时从该用户余额扣除成本';
COMMENT ON COLUMN providers.name IS '服务商名称，用于后台识别和默认展示';
COMMENT ON COLUMN providers.status IS '服务商状态：1 启用，0 禁用';
COMMENT ON COLUMN providers.created_at IS '创建时间，Unix 秒级时间戳';
COMMENT ON COLUMN providers.updated_at IS '更新时间，Unix 秒级时间戳';

COMMENT ON TABLE provider_domains IS '服务商域名绑定表：根据请求 Host 解析到对应服务商';
COMMENT ON COLUMN provider_domains.id IS '域名绑定 ID，主键';
COMMENT ON COLUMN provider_domains.provider_id IS '所属服务商 ID，关联 providers.id';
COMMENT ON COLUMN provider_domains.domain IS '服务商绑定域名，例如 api.example.com；必须全局唯一';
COMMENT ON COLUMN provider_domains.status IS '域名状态：1 已验证可用，0 待验证或禁用';
COMMENT ON COLUMN provider_domains.verify_token IS '域名验证令牌，可用于 TXT 或 CNAME 校验';
COMMENT ON COLUMN provider_domains.created_at IS '创建时间，Unix 秒级时间戳';
COMMENT ON COLUMN provider_domains.updated_at IS '更新时间，Unix 秒级时间戳';

COMMENT ON TABLE provider_configs IS '服务商页面配置表：控制服务商域名下的站点展示';
COMMENT ON COLUMN provider_configs.id IS '配置 ID，主键';
COMMENT ON COLUMN provider_configs.provider_id IS '所属服务商 ID，唯一关联 providers.id';
COMMENT ON COLUMN provider_configs.site_name IS '服务商站点名称，会覆盖前端显示的系统名';
COMMENT ON COLUMN provider_configs.logo IS '服务商 Logo 地址';
COMMENT ON COLUMN provider_configs.theme_color IS '服务商主题色，例如 #1677ff';
COMMENT ON COLUMN provider_configs.login_background IS '登录页背景图地址';
COMMENT ON COLUMN provider_configs.home_modules IS '首页模块开关配置，JSON 字符串';
COMMENT ON COLUMN provider_configs.nav_modules IS '导航菜单开关配置，JSON 字符串';
COMMENT ON COLUMN provider_configs.pricing_display IS '模型价格页展示配置，JSON 字符串';
COMMENT ON COLUMN provider_configs.announcement IS '服务商自定义公告文本';
COMMENT ON COLUMN provider_configs.footer_text IS '页脚文案或 HTML 文本';
COMMENT ON COLUMN provider_configs.support_url IS '客服链接';
COMMENT ON COLUMN provider_configs.wechat_support IS '微信客服';
COMMENT ON COLUMN provider_configs.qq_support IS 'QQ客服';
COMMENT ON COLUMN provider_configs.created_at IS '创建时间，Unix 秒级时间戳';
COMMENT ON COLUMN provider_configs.updated_at IS '更新时间，Unix 秒级时间戳';

COMMENT ON TABLE provider_model_pricings IS '服务商模型定价表：把服务商展示模型映射到主站真实模型，并配置服务商售价';
COMMENT ON COLUMN provider_model_pricings.id IS '服务商模型定价 ID，主键';
COMMENT ON COLUMN provider_model_pricings.provider_id IS '所属服务商 ID，关联 providers.id';
COMMENT ON COLUMN provider_model_pricings.public_model_name IS '服务商对外展示和用户调用的模型名';
COMMENT ON COLUMN provider_model_pricings.base_model_name IS '主站真实模型名，实际中继和上游调用使用该模型';
COMMENT ON COLUMN provider_model_pricings.enabled IS '是否启用该服务商模型';
COMMENT ON COLUMN provider_model_pricings.pricing_type IS '定价方式：ratio 表示按比例，delta 表示在主站价格基础上加减';
COMMENT ON COLUMN provider_model_pricings.ratio IS '比例定价倍数；pricing_type 为 ratio 时使用，例如 1.2 表示主站价格的 1.2 倍';
COMMENT ON COLUMN provider_model_pricings.delta_model_ratio IS '按倍率计费模型的加减值；pricing_type 为 delta 且模型按 ratio 计费时使用';
COMMENT ON COLUMN provider_model_pricings.delta_model_price IS '按固定价格计费模型的加减金额；pricing_type 为 delta 且模型按 price 计费时使用';
COMMENT ON COLUMN provider_model_pricings.created_at IS '创建时间，Unix 秒级时间戳';
COMMENT ON COLUMN provider_model_pricings.updated_at IS '更新时间，Unix 秒级时间戳';

COMMENT ON COLUMN users.provider_id IS '用户所属服务商 ID；0 表示主站用户，非 0 表示对应服务商下的隔离用户';
COMMENT ON COLUMN tokens.provider_id IS '令牌所属服务商 ID；必须与当前访问域名解析出的 provider_id 一致';
COMMENT ON COLUMN top_ups.provider_id IS '充值订单所属服务商 ID；用于把充值金额加到对应服务商用户余额';
COMMENT ON COLUMN logs.provider_id IS '日志所属服务商 ID；0 表示主站日志，非 0 表示服务商域名下产生的日志';
COMMENT ON COLUMN logs.base_model_name IS '服务商场景下的主站真实模型名；普通主站日志可为空';
COMMENT ON COLUMN logs.billing_side IS '服务商账本方向：provider_user 表示服务商用户售价记录，provider_cost 表示服务商主站成本记录';

COMMIT;

-- 初始化示例，请按实际用户 ID 和域名修改后单独执行：
-- INSERT INTO providers (owner_user_id, name, status, created_at, updated_at)
-- VALUES (123, '服务商 A', 1, EXTRACT(EPOCH FROM now())::BIGINT, EXTRACT(EPOCH FROM now())::BIGINT);
--
-- INSERT INTO provider_domains (provider_id, domain, status, verify_token, created_at, updated_at)
-- VALUES (1, 'api.provider-a.com', 1, 'manual', EXTRACT(EPOCH FROM now())::BIGINT, EXTRACT(EPOCH FROM now())::BIGINT);
--
-- INSERT INTO provider_configs (provider_id, site_name, logo, theme_color, login_background, home_modules, nav_modules, pricing_display, announcement, footer_text, support_url, created_at, updated_at)
-- VALUES (1, '服务商 A', '', '#1677ff', '', '{}', '{}', '{}', '', '', '', EXTRACT(EPOCH FROM now())::BIGINT, EXTRACT(EPOCH FROM now())::BIGINT);
--
-- INSERT INTO provider_model_pricings (provider_id, public_model_name, base_model_name, enabled, pricing_type, ratio, created_at, updated_at)
-- VALUES (1, 'gpt-4o-provider', 'gpt-4o', true, 'ratio', 1.2, EXTRACT(EPOCH FROM now())::BIGINT, EXTRACT(EPOCH FROM now())::BIGINT);



-- PostgreSQL migration for provider-scoped redemption codes.
-- Main site redemption codes use provider_id = 0.
-- Each service provider owns an isolated redemption-code inventory by provider_id.

ALTER TABLE redemptions
    ADD COLUMN IF NOT EXISTS provider_id integer NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_redemptions_provider_id
    ON redemptions (provider_id);

-- Drop old unique indexes that only constrain the key column globally.
-- The new uniqueness boundary is provider_id + key.
DO $$
DECLARE
idx record;
BEGIN
FOR idx IN
SELECT i.relname AS index_name
FROM pg_class t
         JOIN pg_index ix ON t.oid = ix.indrelid
         JOIN pg_class i ON i.oid = ix.indexrelid
         JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = ANY(ix.indkey)
WHERE t.relname = 'redemptions'
  AND ix.indisunique = true
  AND array_length(ix.indkey, 1) = 1
  AND a.attname = 'key'
    LOOP
        EXECUTE format('DROP INDEX IF EXISTS %I', idx.index_name);
END LOOP;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS ux_provider_redemption_key
    ON redemptions (provider_id, "key");

-- Optional backfill: if you already have redeemed records and want provider_id
-- aligned from the used user, run this once after users.provider_id exists.
UPDATE redemptions r
SET provider_id = u.provider_id
    FROM users u
WHERE r.used_user_id = u.id
  AND r.provider_id = 0
  AND COALESCE(u.provider_id, 0) > 0;




-- PostgreSQL 服务商奖励隔离迁移脚本。

CREATE TABLE IF NOT EXISTS provider_reward_configs (
                                                       id SERIAL PRIMARY KEY,
                                                       provider_id integer NOT NULL,
                                                       quota_for_new_user integer NOT NULL DEFAULT 0,
                                                       quota_for_inviter integer NOT NULL DEFAULT 0,
                                                       quota_for_invitee integer NOT NULL DEFAULT 0,
                                                       checkin_enabled boolean NOT NULL DEFAULT false,
                                                       checkin_min_quota integer NOT NULL DEFAULT 0,
                                                       checkin_max_quota integer NOT NULL DEFAULT 0,
                                                       invite_topup_rebate_ratio numeric(10,6) NOT NULL DEFAULT 0,
    invite_consume_rebate_ratio_level2 numeric(10,6) NOT NULL DEFAULT 0,
    created_at bigint,
    updated_at bigint
    );
CREATE UNIQUE INDEX IF NOT EXISTS ux_provider_reward_configs_provider_id
    ON provider_reward_configs(provider_id);

CREATE TABLE IF NOT EXISTS reward_records (
                                              id SERIAL PRIMARY KEY,
                                              provider_id integer NOT NULL DEFAULT 0,
                                              user_id integer NOT NULL,
                                              source_type varchar(32) NOT NULL,
    source_id integer NOT NULL DEFAULT 0,
    quota integer NOT NULL,
    description varchar(255) NOT NULL DEFAULT '',
    created_at bigint
    );
CREATE INDEX IF NOT EXISTS idx_reward_records_provider_id ON reward_records(provider_id);
CREATE INDEX IF NOT EXISTS idx_reward_records_user_id ON reward_records(user_id);
CREATE INDEX IF NOT EXISTS idx_reward_records_created_at ON reward_records(created_at);
CREATE INDEX IF NOT EXISTS idx_reward_records_source ON reward_records(source_type, source_id);
CREATE UNIQUE INDEX IF NOT EXISTS ux_reward_records_source_user
    ON reward_records(provider_id, source_type, source_id, user_id);

ALTER TABLE users ADD COLUMN IF NOT EXISTS provider_id integer NOT NULL DEFAULT 0;
ALTER TABLE users DROP CONSTRAINT IF EXISTS ux_user_provider_aff;
DROP INDEX IF EXISTS ux_user_provider_aff;
CREATE UNIQUE INDEX IF NOT EXISTS ux_user_provider_aff ON users(provider_id, aff_code);

ALTER TABLE invite_records ADD COLUMN IF NOT EXISTS provider_id integer NOT NULL DEFAULT 0;
ALTER TABLE checkins ADD COLUMN IF NOT EXISTS provider_id integer NOT NULL DEFAULT 0;
ALTER TABLE redemptions ADD COLUMN IF NOT EXISTS provider_id integer NOT NULL DEFAULT 0;
ALTER TABLE consume_rebates ADD COLUMN IF NOT EXISTS provider_id integer NOT NULL DEFAULT 0;
ALTER TABLE topup_rebates ADD COLUMN IF NOT EXISTS provider_id integer NOT NULL DEFAULT 0;

DROP INDEX IF EXISTS idx_user_checkin_date;
CREATE UNIQUE INDEX IF NOT EXISTS idx_provider_user_checkin_date
    ON checkins(provider_id, user_id, checkin_date);

DROP INDEX IF EXISTS idx_redemptions_key;
CREATE UNIQUE INDEX IF NOT EXISTS ux_provider_redemption_key
    ON redemptions(provider_id, "key");

CREATE INDEX IF NOT EXISTS idx_invite_records_provider_id ON invite_records(provider_id);
CREATE INDEX IF NOT EXISTS idx_checkins_provider_id ON checkins(provider_id);
CREATE INDEX IF NOT EXISTS idx_redemptions_provider_id ON redemptions(provider_id);
CREATE INDEX IF NOT EXISTS idx_consume_rebates_provider_id ON consume_rebates(provider_id);
CREATE INDEX IF NOT EXISTS idx_topup_rebates_provider_id ON topup_rebates(provider_id);

UPDATE invite_records ir
SET provider_id = u.provider_id
    FROM users u
WHERE u.id = ir.inviter_id AND ir.provider_id = 0;

UPDATE checkins c
SET provider_id = u.provider_id
    FROM users u
WHERE u.id = c.user_id AND c.provider_id = 0;

UPDATE redemptions r
SET provider_id = u.provider_id
    FROM users u
WHERE u.id = r.used_user_id AND r.provider_id = 0;

UPDATE consume_rebates cr
SET provider_id = u.provider_id
    FROM users u
WHERE u.id = cr.inviter_id AND cr.provider_id = 0;

UPDATE topup_rebates tr
SET provider_id = u.provider_id
    FROM users u
WHERE u.id = tr.inviter_id AND tr.provider_id = 0;

INSERT INTO reward_records (provider_id, user_id, source_type, source_id, quota, description, created_at)
SELECT
    ir.provider_id,
    ir.inviter_id,
    'inviter_reward',
    ir.invitee_id,
    ir.reward_quota,
    'inviter reward',
    COALESCE(ir.created_at, EXTRACT(EPOCH FROM NOW())::bigint)
FROM invite_records ir
WHERE ir.provider_id > 0 AND ir.reward_quota > 0
    ON CONFLICT DO NOTHING;

INSERT INTO reward_records (provider_id, user_id, source_type, source_id, quota, description, created_at)
SELECT
    c.provider_id,
    c.user_id,
    'checkin',
    c.id,
    c.quota_awarded,
    'checkin reward',
    COALESCE(c.created_at, EXTRACT(EPOCH FROM NOW())::bigint)
FROM checkins c
WHERE c.provider_id > 0 AND c.quota_awarded > 0
    ON CONFLICT DO NOTHING;

INSERT INTO reward_records (provider_id, user_id, source_type, source_id, quota, description, created_at)
SELECT
    r.provider_id,
    r.used_user_id,
    'redemption',
    r.id,
    r.quota,
    'redemption reward',
    COALESCE(r.redeemed_time, r.created_time, EXTRACT(EPOCH FROM NOW())::bigint)
FROM redemptions r
WHERE r.provider_id > 0 AND r.status = 3 AND r.quota > 0
    ON CONFLICT DO NOTHING;

INSERT INTO reward_records (provider_id, user_id, source_type, source_id, quota, description, created_at)
SELECT
    cr.provider_id,
    cr.inviter_id,
    'consume_rebate',
    cr.id,
    cr.rebate_quota,
    'invite consume rebate',
    COALESCE(cr.created_at, EXTRACT(EPOCH FROM NOW())::bigint)
FROM consume_rebates cr
WHERE cr.provider_id > 0 AND cr.rebate_quota > 0
    ON CONFLICT DO NOTHING;

INSERT INTO reward_records (provider_id, user_id, source_type, source_id, quota, description, created_at)
SELECT
    tr.provider_id,
    tr.inviter_id,
    'topup_rebate',
    tr.id,
    tr.rebate_quota,
    'invite topup rebate',
    COALESCE(tr.created_at, EXTRACT(EPOCH FROM NOW())::bigint)
FROM topup_rebates tr
WHERE tr.provider_id > 0 AND tr.rebate_quota > 0
    ON CONFLICT DO NOTHING;

COMMENT ON TABLE provider_reward_configs IS '服务商奖励策略配置表';
COMMENT ON COLUMN provider_reward_configs.id IS '主键 ID';
COMMENT ON COLUMN provider_reward_configs.provider_id IS '服务商 ID';
COMMENT ON COLUMN provider_reward_configs.quota_for_new_user IS '新用户注册赠送额度';
COMMENT ON COLUMN provider_reward_configs.quota_for_inviter IS '邀请人注册奖励额度';
COMMENT ON COLUMN provider_reward_configs.quota_for_invitee IS '被邀请人注册奖励额度';
COMMENT ON COLUMN provider_reward_configs.checkin_enabled IS '是否启用签到奖励';
COMMENT ON COLUMN provider_reward_configs.checkin_min_quota IS '签到奖励最小额度';
COMMENT ON COLUMN provider_reward_configs.checkin_max_quota IS '签到奖励最大额度';
COMMENT ON COLUMN provider_reward_configs.invite_topup_rebate_ratio IS '邀请充值返利比例';
COMMENT ON COLUMN provider_reward_configs.invite_consume_rebate_ratio_level2 IS '二级邀请消费返利比例';
COMMENT ON COLUMN provider_reward_configs.created_at IS '创建时间戳';
COMMENT ON COLUMN provider_reward_configs.updated_at IS '更新时间戳';

COMMENT ON TABLE reward_records IS '服务商维度奖励流水表';
COMMENT ON COLUMN reward_records.id IS '主键 ID';
COMMENT ON COLUMN reward_records.provider_id IS '服务商 ID';
COMMENT ON COLUMN reward_records.user_id IS '奖励接收用户 ID';
COMMENT ON COLUMN reward_records.source_type IS '奖励来源类型：新用户、邀请人奖励、被邀请人奖励、签到、兑换码、消费返利、充值返利';
COMMENT ON COLUMN reward_records.source_id IS '来源业务记录 ID';
COMMENT ON COLUMN reward_records.quota IS '奖励额度';
COMMENT ON COLUMN reward_records.description IS '奖励说明';
COMMENT ON COLUMN reward_records.created_at IS '创建时间戳';

COMMENT ON COLUMN users.provider_id IS '所属服务商 ID，0 表示主站用户';
COMMENT ON COLUMN invite_records.provider_id IS '所属服务商 ID，0 表示主站邀请记录';
COMMENT ON COLUMN checkins.provider_id IS '所属服务商 ID，0 表示主站签到记录';
COMMENT ON COLUMN redemptions.provider_id IS '所属服务商 ID，0 表示主站兑换码';
COMMENT ON COLUMN consume_rebates.provider_id IS '所属服务商 ID，0 表示主站消费返利记录';
COMMENT ON COLUMN topup_rebates.provider_id IS '所属服务商 ID，0 表示主站充值返利记录';





-- 加密货币支付
DROP TABLE IF EXISTS "crypto_transactions";

CREATE SEQUENCE IF NOT EXISTS "crypto_transactions_id_seq";

CREATE TABLE "crypto_transactions" (
                                       "id" INT8 NOT NULL DEFAULT nextval('crypto_transactions_id_seq' :: REGCLASS),
                                       "top_up_id" INT8,
                                       "subscription_order_id" INT8,
                                       "user_id" INT8,
                                       "trade_no" VARCHAR (255),
                                       "tx_hash" VARCHAR (128),
                                       "chain_id" INT8,
                                       "token_symbol" VARCHAR (20),
                                       "token_contract" VARCHAR (128),
                                       "receiver_address" VARCHAR (128),
                                       "payer_address" VARCHAR (128),
                                       "usdt_amount" VARCHAR (64),
                                       "block_number" INT8 DEFAULT 0,
                                       "confirmations" INT8 DEFAULT 0,
                                       "status" VARCHAR (20),
                                       "create_time" INT8,
                                       "complete_time" INT8,
                                       "updated_at" TIMESTAMPTZ (6)
);

-- 表注释
COMMENT ON TABLE "crypto_transactions" IS '加密货币链上交易记录';

-- 字段注释
COMMENT ON COLUMN "crypto_transactions"."id"                      IS '主键';
COMMENT ON COLUMN "crypto_transactions"."top_up_id"               IS '关联的充值订单 ID';
COMMENT ON COLUMN "crypto_transactions"."subscription_order_id"   IS '关联的订阅订单 ID';
COMMENT ON COLUMN "crypto_transactions"."user_id"                 IS '用户 ID';
COMMENT ON COLUMN "crypto_transactions"."trade_no"                IS '订单号（唯一）';
COMMENT ON COLUMN "crypto_transactions"."tx_hash"                 IS '链上交易哈希（确认后填入，唯一）';
COMMENT ON COLUMN "crypto_transactions"."chain_id"                IS '链 ID（如 BSC 主网为 56）';
COMMENT ON COLUMN "crypto_transactions"."token_symbol"            IS '代币符号（如 USDT）';
COMMENT ON COLUMN "crypto_transactions"."token_contract"          IS '代币合约地址';
COMMENT ON COLUMN "crypto_transactions"."receiver_address"        IS '收款地址';
COMMENT ON COLUMN "crypto_transactions"."payer_address"           IS '付款地址（链上确认后填入）';
COMMENT ON COLUMN "crypto_transactions"."usdt_amount"             IS 'USDT 金额（字符串存储，保证精度）';
COMMENT ON COLUMN "crypto_transactions"."block_number"            IS '区块号（确认后填入）';
COMMENT ON COLUMN "crypto_transactions"."confirmations"           IS '确认数（确认后填入）';
COMMENT ON COLUMN "crypto_transactions"."status"                  IS '状态：pending-待确认 / success-已完成 / failed-失败';
COMMENT ON COLUMN "crypto_transactions"."create_time"             IS '创建时间（Unix 时间戳）';
COMMENT ON COLUMN "crypto_transactions"."complete_time"           IS '完成时间（Unix 时间戳）';
COMMENT ON COLUMN "crypto_transactions"."updated_at"              IS '记录更新时间';



DROP TABLE IF EXISTS "crypto_chain_config";

CREATE TABLE "crypto_chain_config" (
                                                "network"           varchar(32) NOT NULL,
                                                "chain_id"          int8 NOT NULL,
                                                "token_symbol"      varchar(20) NOT NULL DEFAULT 'USDT',
                                                "token_decimals"    int2 NOT NULL DEFAULT 18,
                                                "token_contract"    varchar(128) NOT NULL DEFAULT '',
                                                "receiver_address"  varchar(128) NOT NULL DEFAULT '',
                                                "rpc_url"           varchar(512) NOT NULL DEFAULT '',
                                                "min_confirmations" int2 NOT NULL DEFAULT 3,
                                                PRIMARY KEY ("network", "token_symbol")
);

-- 表注释
COMMENT ON TABLE "crypto_chain_config" IS '加密货币链配置表：每条链每种代币一行，network + token_symbol 唯一确定一组参数';

-- 字段注释
COMMENT ON COLUMN "crypto_chain_config"."network"           IS '网络名称（复合主键），如 Sepolia / BSC / Polygon，大小写不敏感匹配';
COMMENT ON COLUMN "crypto_chain_config"."chain_id"          IS 'EIP-155 链 ID（Sepolia=11155111, BSC=97, Polygon=137）';
COMMENT ON COLUMN "crypto_chain_config"."token_symbol"      IS '代币符号（复合主键），如 USDT / USDC';
COMMENT ON COLUMN "crypto_chain_config"."token_decimals"    IS '代币精度（Sepolia MockUSDT=6, BSC USDT=18, Polygon USDT=6）';
COMMENT ON COLUMN "crypto_chain_config"."token_contract"    IS '代币合约地址';
COMMENT ON COLUMN "crypto_chain_config"."receiver_address"  IS '收款钱包地址';
COMMENT ON COLUMN "crypto_chain_config"."rpc_url"           IS '链节点 RPC 地址（含 API Key）';
COMMENT ON COLUMN "crypto_chain_config"."min_confirmations" IS '最小链上确认数（测试网建议 1~2，主网建议 ≥3）';


INSERT INTO "options" ("key", "value") VALUES
                                                    ('CryptoUSDtoTokenRate', '1'),
                                                    ('CryptoCNYtoTokenRate', '0.1471');



ALTER TABLE checkins ADD COLUMN IF NOT EXISTS provider_id integer NOT NULL DEFAULT 0;
ALTER TABLE invite_records ADD COLUMN IF NOT EXISTS provider_id integer NOT NULL DEFAULT 0;




BEGIN;

ALTER TABLE provider_configs
    ADD COLUMN IF NOT EXISTS secondary_color VARCHAR(32) NOT NULL DEFAULT '';

COMMENT ON COLUMN provider_configs.theme_color IS '服务商站点主色，为空时访问服务商站点使用默认主色 #09FEF7';
COMMENT ON COLUMN provider_configs.secondary_color IS '服务商站点辅色，为空时访问服务商站点使用默认辅色 #BAFF29';

COMMIT;



BEGIN;

ALTER TABLE provider_model_pricings
    ADD COLUMN IF NOT EXISTS consume_rebate_ratio_level1 numeric(10,6),
    ADD COLUMN IF NOT EXISTS consume_rebate_ratio_level2 numeric(10,6);

UPDATE provider_model_pricings
SET
    consume_rebate_ratio_level1 = COALESCE(consume_rebate_ratio_level1, 0),
    consume_rebate_ratio_level2 = COALESCE(consume_rebate_ratio_level2, 0);

ALTER TABLE provider_model_pricings
    ALTER COLUMN consume_rebate_ratio_level1 SET DEFAULT 0,
ALTER COLUMN consume_rebate_ratio_level2 SET DEFAULT 0,
  ALTER COLUMN consume_rebate_ratio_level1 SET NOT NULL,
  ALTER COLUMN consume_rebate_ratio_level2 SET NOT NULL;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'chk_provider_model_pricings_rebate_l1_range'
  ) THEN
ALTER TABLE provider_model_pricings
    ADD CONSTRAINT chk_provider_model_pricings_rebate_l1_range
        CHECK (consume_rebate_ratio_level1 >= 0 AND consume_rebate_ratio_level1 <= 100);
END IF;

  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'chk_provider_model_pricings_rebate_l2_range'
  ) THEN
ALTER TABLE provider_model_pricings
    ADD CONSTRAINT chk_provider_model_pricings_rebate_l2_range
        CHECK (consume_rebate_ratio_level2 >= 0 AND consume_rebate_ratio_level2 <= 100);
END IF;
END $$;

ALTER TABLE consume_rebates
    ADD COLUMN IF NOT EXISTS provider_pricing_id integer NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS public_model_name varchar(255) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS base_model_name varchar(255) NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_consume_rebates_provider_pricing_id
    ON consume_rebates (provider_pricing_id);

CREATE INDEX IF NOT EXISTS idx_consume_rebates_provider_model
    ON consume_rebates (provider_id, public_model_name);

COMMIT;


-- 服务商提现表------------------------------
DROP TABLE IF EXISTS "provider_withdraw";

CREATE SEQUENCE IF NOT EXISTS "provider_withdraw_id_seq";

CREATE TABLE "provider_withdraw" (
  "id"                    int8          NOT NULL DEFAULT nextval('provider_withdraw_id_seq'::regclass),
  "provider_id"           int8,
  "amount"                numeric(18,8) NOT NULL DEFAULT 0,
  "currency"              varchar(20),
  "usd_amount"            numeric(18,8) NOT NULL DEFAULT 0,
  "cny_amount"            numeric(18,8) NOT NULL DEFAULT 0,
  "usd_to_cny_rate"       numeric(18,8) NOT NULL DEFAULT 0,
  "status"                int4          NOT NULL DEFAULT 0,
  "created_at"            int8          NOT NULL DEFAULT 0,
  "updated_at"            int8          NOT NULL DEFAULT 0
);

-- 表注释
COMMENT ON TABLE "provider_withdraw" IS '服务商提现表';

-- 字段注释
COMMENT ON COLUMN "provider_withdraw"."id"                      IS '主键';
COMMENT ON COLUMN "provider_withdraw"."provider_id"             IS '服务商id';
COMMENT ON COLUMN "provider_withdraw"."amount"                  IS '金额';
COMMENT ON COLUMN "provider_withdraw"."currency"                IS '货币';
COMMENT ON COLUMN "provider_withdraw"."usd_amount"              IS '美元金额';
COMMENT ON COLUMN "provider_withdraw"."cny_amount"              IS '人民币金额';
COMMENT ON COLUMN "provider_withdraw"."usd_to_cny_rate"         IS '美元到人民币汇率';
COMMENT ON COLUMN "provider_withdraw"."status"                  IS '状态: 1-审核中, 2-审核通过, 3-已拒绝, 4-已取消';
COMMENT ON COLUMN "provider_withdraw"."created_at"              IS '创建时间';
COMMENT ON COLUMN "provider_withdraw"."updated_at"              IS '更新时间';






-- 完善服务利润表
BEGIN;
ALTER TABLE provider_profits
    ADD COLUMN IF NOT EXISTS gross_profit_quota bigint NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS rebate_quota bigint NOT NULL DEFAULT 0;

COMMENT ON COLUMN provider_profits.gross_profit_quota IS '分佣前毛利润';
COMMENT ON COLUMN provider_profits.rebate_quota IS '一级 + 二级总分佣';
COMMIT;


ALTER TABLE provider_configs
    ADD COLUMN IF NOT EXISTS import_price_ratio numeric(10,6);
COMMENT ON COLUMN provider_configs.import_price_ratio IS '进口价比例';

UPDATE provider_configs
SET import_price_ratio = 1
WHERE import_price_ratio IS NULL
   OR import_price_ratio <= 0
   OR import_price_ratio > 1;

ALTER TABLE provider_configs
    ALTER COLUMN import_price_ratio SET DEFAULT 1;

ALTER TABLE provider_configs
    ALTER COLUMN import_price_ratio SET NOT NULL;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conname = 'chk_provider_configs_import_price_ratio'
  ) THEN
ALTER TABLE provider_configs
    ADD CONSTRAINT chk_provider_configs_import_price_ratio
        CHECK (import_price_ratio > 0 AND import_price_ratio <= 1);
END IF;
END $$;


-- 版本更新日志表------------------------------------

DROP TABLE IF EXISTS "version_log";

CREATE SEQUENCE IF NOT EXISTS "version_log_id_seq";

CREATE TABLE "version_log" (
  "id"                    int8          NOT NULL DEFAULT nextval('version_log_id_seq'::regclass),
  "version"               varchar(64),
  "log"                   text,
  "created_at"            int8          NOT NULL DEFAULT 0,
  "updated_at"            int8          NOT NULL DEFAULT 0  
);

COMMENT ON TABLE "version_log" IS '更新日志表';

COMMENT ON COLUMN "version_log"."id"                      IS '主键';
COMMENT ON COLUMN "version_log"."version"                 IS '版本号';
COMMENT ON COLUMN "version_log"."log"                     IS '日志内容';
COMMENT ON COLUMN "version_log"."created_at"              IS '创建时间';
COMMENT ON COLUMN "version_log"."updated_at"              IS '更新时间';
