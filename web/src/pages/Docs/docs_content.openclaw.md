本手册聚焦AllRouter.AI接入飞书妙搭OpenClaw的全流程实操，致力于帮助所有用户快速完成对接部署，高效实现AllRouter.AI与飞书妙搭OpenClaw的稳定互通，降低接入门槛、提升开发效率，为相关业务场景及用户提供专业、可落地的技术指导。

# 一、开通 AllRouter

## 注册AllRouter账号并登录

访问 [https://allrouter.ai/](https://allrouter.ai/)，点击上方【登录】→【注册账号】，在弹出的页面中依次输入用户名和密码，最后点击【立即注册】，即完成用户账号的注册。

然后，使用该账号登录，点击【立即登录】即可成功登录AllRouter.AI。

![image 0](/docs_images/openclaw_0.png)

![image 1](/docs_images/openclaw_1.png)

## 查询和选择模型

功能描述：【模型广场】是 AllRouter.AI 的核心资源库，不仅展示了平台支持的所有 AI 模型，还提供了透明、实时的计费查询功能。用户可以根据官方倍率或充值后的实际成本，精确评估每一款模型的调用代价。

1. 第 1 步： 在页面顶部导航栏中，点击【模型广场】。进入模型列表，默认以卡片形式展示模型 ID 及对应的 1M Tokens 价格。
2. 第 2 步： 掌握两种计费模式。

- 按量计费（Tokens）： 针对文本对话模型，根据输入、补全及缓存读取的 Tokens 数量计费。
- 按次计费（Requests）： 针对绘图、任务等特定模型，每发起一次请求扣除固定费用。

1. 第 3 步： 灵活切换价格显示方式。

- 充值价格显示： 开启此开关，系统将根据您的充值比例自动换算并显示实际扣除的金额。
- 倍率显示： 开启此开关，模型卡片将显示该模型相对于官方价格的计费倍率。

1. 第 4 步： 利用侧边栏精准定位模型。在左侧【供应商】区域点击特定图标，或在【计费类型】中选择【按量计费】。
2. 第 5 步： 搜索与复制模型 ID。在顶部搜索框输入关键字，找到目标后点击卡片右上角的【复制】图标。
3. 第 6 步： 批量操作与视图切换。

- 表格视图： 点击右上角【表格视图】，以更紧凑的列表形式对比多个模型的价格。
- 预期结果： 极大地提升开发者在多模型环境下的配置效率。

## 创建令牌，获取 Base_URL、APIKey、ModelName 参数

要开始调用API，您需要创建一个令牌:

1. 第1步:点击左侧菜单栏的【令牌管理】。
2. 第2步:点击【添加令牌】按钮。
3. 第3步:设置令牌名称、令牌分组（一定要设置令牌分组）、额度限制、过期时间和访问限制等。

![image 2](/docs_images/openclaw_2.png)

![image 3](/docs_images/openclaw_3.png)

1. 第4步:创建成功后，复制生成的令牌\(Key\)，即可在您的应用中使用。
2. 第5步：复制并保存base\_url、api\_key、ModelName的值。如下：

base\_url=https://allrouter.ai/v1（该模型分组下的模型均可使用这个baseurl）

api\_key=sk\-xxxxxxxxxxxxxxxxxxxxxxxxxx

ModelName（modelID）=gpt\-5.4（可在模型广场中直接复制）

![image 4](/docs_images/openclaw_4.png)

![image 5](/docs_images/openclaw_5.png)

![image 6](/docs_images/openclaw_6.png)

# 二、搭建飞书妙搭 OpenClaw 应用

## 1.登录飞书妙搭开发平台

访问飞书妙搭官方开发平台网址（[https://miaoda.feishu.cn/](https://miaoda.feishu.cn/)），点击“前往开发”。

![image 7](/docs_images/openclaw_7.png)

## 一键部署 OpenClaw

![image 8](/docs_images/openclaw_8.png)

## 开始创建

![image 9](/docs_images/openclaw_9.png)

## 4.点击“继续”

![image 10](/docs_images/openclaw_10.png)

## 5.自定义设置龙虾智能体名称

如“工作助理”、“小助手”等等。

![image 11](/docs_images/openclaw_11.png)

## 6.等待创建完成

等待openclaw创建完成即可。

![image 12](/docs_images/openclaw_12.png)

三、将AllRouter参数信息配置到飞书龙虾中

## 1.进入到飞书龙虾管理后台

接上一步骤，打开龙虾应用，进入到飞书openclaw管理后台

![image 13](/docs_images/openclaw_13.png)

![image 14](/docs_images/openclaw_14.png)

## 配置 base_url、api_key、modelID 参数信息

切换到“设置”页面，在可用模型中点击“自定义模型”，在弹窗中依次填写步骤一种的 base\_url\\api\_key\\modelID 参数值。

- 提供方（Provider）：大模型提供方名称；
- 接口地址（Endpoint）：base\_url 的值；
- 秘钥（API Key）：api\_key 的值；
- 模型 ID（Model ID）：modelname的值。

![image 15](/docs_images/openclaw_15.png)

填写效果如下图：

![image 16](/docs_images/openclaw_16.png)

![image 17](/docs_images/openclaw_17.png)

## 切换模型

![image 18](/docs_images/openclaw_18.png)

四、测试效果

## 1.打开飞书智能体

![image 19](/docs_images/openclaw_19.png)

## 拉起飞书

![image 20](/docs_images/openclaw_20.png)

## 对话测试

在飞书页面中，向openclaw助手发消息进行对话测试，如“你好”、“你能做什么”、“你有什么能力”等等。

![image 21](/docs_images/openclaw_21.png)
