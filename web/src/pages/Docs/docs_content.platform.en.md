## How to Use Redemption Codes

### 1.Create Redemption Codes

Prerequisite: Logged in with an administrator account.

（1）Navigate to Console - "Service Provider" - "Redemption Code Management", then click "Add Redemption Code" to create a new redemption code.

![image](/docs_images/platform_en_1.png)

（2）Configure the redemption code information on the side panel:

- Name: Custom redemption code name, such as "Existing User Referral Reward" or "Gift Card $50 Voucher".
- Expiration Time: Set the validity period of the redemption code.
- Amount: Redemption value of the code.
- Generate quantity: Number of redemption codes to be generated.

After completing the configuration, click "Submit" and then "Confirm" to finish creating the redemption code. The generated redemption code file (*.txt) can be downloaded and distributed to users for redemption.

![image](/docs_images/platform_en_2.png)

![image](/docs_images/platform_en_3.png)

![image](/docs_images/platform_en_4.png)

### 2.Redeem a Redemption Code

Prerequisite: Logged in with a user account and obtained a redemption code.

（1）Navigate to Console - "Marketing Campaigns" - "Redemption Codes". Copy the received redemption code and enter it into the redemption code field. Click "Redeem Now" to complete the redemption process. After redemption, the balance can be used immediately.

![image](/docs_images/platform_en_5.png)

![image](/docs_images/platform_en_6.png)

（2）After the redemption is completed, users can check the redemption code status under "My Redemption Records".

![image](/docs_images/platform_en_7.png)

## How to Configure Model Pricing

Prerequisite: Logged in with a provider account.

### 1.Select a Model

（1）Navigate to: Console - "Service Provider" - "Provider Management" - "Model Pricing". Then click "Add Pricing" on the pop-up page.

![image](/docs_images/platform_en_8.png)

![image](/docs_images/platform_en_9_1.png)

Alternatively, navigate to "Model Pricing", select the target model from the pop-up page, and click "Edit". You can then directly adjust the Markup Rate, Level-1 Consumption Commission Rate (Profit Sharing Ratio), and Level-2 Consumption Commission Rate (Profit Sharing Ratio) as needed.

![image](/docs_images/platform_en_9_2.png)

![image](/docs_images/platform_en_9_3.png)

### 2.Configure Model Pricing

On the configuration page, set the following information:

- Model Name Displayed to Provider Users: Enter the model name.
- Channel Pricing: Displays model pricing across different channels, including: Group, Billing Type, Input Price, Completion Price, Cache Read Price, Cache Write Price, Provider Discount.
- Enable: Turn on the enable switch.
- Pricing Method:
  - Markup by Percentage: The markup is calculated based on the provider's discounted price rather than the platform's original price. Final Selling Price = Platform Original Price × Discount / 10 × (1 + Markup Percentage). The markup percentage can be customized.
  - Fixed Markup: A fixed amount is added on top of the provider's discounted price. Final Selling Price = Provider Discounted Price + Fixed Markup. Token-based model markup multipliers and per-request model markup amounts can be customized.
- Level-1 Consumption Commission Rate (Profit Sharing Ratio): Customizable.
- Level-2 Consumption Commission Rate (Profit Sharing Ratio): Customizable.

After completing the configuration, click "Confirm" to save the settings.

![image](/docs_images/platform_en_10.png)

## How to View Dashboard Data

Prerequisite: Logged in with either an administrator account or a standard user account.

### 1.Dashboard Overview

（1）Navigate to Console - "Dashboard" to view the following information:

- Remaining balance after the last 24 hours of consumption.
- Statistics on large-model API requests and request counts within the last 24 hours.
- Total credits consumed and total tokens used within the last 24 hours.
- Average RPM (Requests Per Minute) and TPM (Tokens Per Minute) performance metrics.
- Bar charts showing model resource consumption and request trends over the past 24 hours.
- Model usage popularity rankings across the platform.
- Model analytics, including: credit consumption share by model, consumption trend analysis, request distribution, request volume rankings.
- Tutorials for configuring commonly used models, including: OpenAI Configuration Guide, Claude Code User Guide, Codex CLI User Guide.

![image](/docs_images/platform_en_11.png)

![image](/docs_images/platform_en_12.png)

## How to View Consumption Details

Prerequisite: Logged in with a user account.

### 1.Usage Logs

Navigate to: Console - "Operation Logs" - "Usage Logs". Here, you can view and analyze detailed API invocation data and real-time usage status.

![image](/docs_images/platform_en_13.png)

### 2.Consumption Details

Under: Console - "Operation Logs" - "Usage Logs". Expand a usage record to view: request log details, billing process, consumption amount for each request.

![image](/docs_images/platform_en_14.png)

## How to View Referral Rewards

Prerequisite: Logged in with a user account.

Navigate to: Console - "Marketing Campaigns" - "Invite reward". You can view: pending withdrawable earnings, total accumulated earnings, number of successfully invited users, referral details (Invited User / Registration Time / Registration Reward / Consumption Rebate).

![image](/docs_images/platform_en_15.png)
