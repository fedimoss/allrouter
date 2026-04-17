AllRouter.AI is a unified large language model \(LLM\) gateway designed to provide convenient, efficient, and cost\-effective access to AI models. This manual introduces the platform’s core functionalities and usage processes from the perspective of general users, helping you quickly understand and utilize the platform.

# Platform Overview

AllRouter.AI integrates more than 50 well\-known large models, including OpenAI, Claude, and Llama. Through a unified API interface, it enables users to access the full range of AI capabilities without switching between multiple platforms.

Key Advantages:

- Seamless Replacement: 100% compatible with the OpenAI SDK. You only need to modify the base\_url to integrate.
- Intelligent Routing: Automatically selects models with lower cost or faster response times based on predefined rules.
- High Availability: Supports automatic failover to ensure uninterrupted service.
- Real\-Time Monitoring: The console provides detailed statistics on latency, token usage, and cost.

# Registration and Login

## Registration

Visit [https://allrouter.ai/](https://allrouter.ai/) and click "[Sign in](https://allrouter.ai/login)" → "Register Account" at the top of the page.

In the pop\-up window, enter your Email and password, then click "Register Now" to complete the account registration process.

![image 0](/docs_images/img_en_0.png)

![image 1](/docs_images/img_en_1.png)

## Login

Visit [https://allrouter.ai/](https://allrouter.ai/) and click "[Sign in](https://allrouter.ai/login)" at the top of the page. Enter your email and password, then click "Login Now " to successfully log in AllRouter.AI.

![image 2](/docs_images/img_en_2.png)

# Dashboard

After logging into the console, you will first see the dashboard. It provides an intuitive overview of your account status:

- Current Balance: View your remaining account credits in real time.
- Usage Statistics: Includes request count, credit consumption, number of abnormal requests, average response time, and token usage.
- Model Analytics: Visualized charts displaying consumption distribution across models, consumption trends, model invocation distribution, as well as usage and resource allocation statistics.

![image 3](/docs_images/img_en_3.png)

![image 4](/docs_images/img_en_4.png)

# Playground

Function Description:Interactively test various AI models directly in the web interface without writing any code.

Steps:

Step 1: Click the "Playground" option in the left sidebar.

Step 2: In the "Model Configuration" panel on the left, select the target model \(e.g., gpt\-5\) from the "Model" dropdown list.

Step 3: Adjust the "Temperature" slider to control the randomness of the output, and configure other parameter settings as needed.

Step 4: Enter your question in the text box at the bottom of the conversation area on the right.

Step 5: Click the "Send" button on the right side of the input box.

Prerequisite:

Your account balance must be greater than 0.

Expected Result:

The AI will return a response immediately, and the consumption details will be displayed below the response.

![image 5](/docs_images/img_en_5.png)

# Token Management

Function Description:Manage the keys \(Tokens\) used for API calls. The platform supports usage limits and expiration settings for each token.

To start calling the API, you need to create a token:

Step 1: Click "Token Management" in the left sidebar.

Step 2: Click the "Create Token" button.

Step 3: Set the token name, usage limit, expiration time, and access restrictions.

Step 4: After the token is successfully created, copy the generated Token \(Key\) and use it in your application.

![image 6](/docs_images/img_en_6.png)

![image 7](/docs_images/img_en_7.png)

# Model Marketplace

Function Description:The Model Marketplace is the core resource hub of AllRouter.AI. It not only showcases all AI models supported by the platform but also provides transparent, real\-time billing information. Users can accurately evaluate the cost of using each model based on the official rate or the actual cost after account top\-up.

Step 1: Click "Model Marketplace" in the top navigation bar. You will enter the model list, where models are displayed as cards by default, showing the Model ID and the price per 1M tokens.

Step 2: Understand the two billing modes:

- Usage\-Based Billing \(Tokens\): For text\-based conversation models, charges are based on the number of tokens used for input, completion, and cached retrieval.
- Request\-Based Billing \(Requests\): For specific models such as drawing or task models, a fixed fee is charged per request.

Step 3: Switch the price display mode flexibly:

- Recharge Price Display: Enable this toggle to automatically convert and display the actual amount deducted based on your account’s top\-up ratio.
- Rate Multiplier Display: Enable this toggle to show the model card’s price multiplier relative to the official price.

Step 4: Use the sidebar to locate models precisely. Click specific icons in the "Vendors" section on the left, or select "Usage\-Based Billing" under Billing Type.

Step 5: Search and copy Model IDs. Enter keywords in the top search box, and after finding the target model, click the "Copy" icon at the top\-right corner of the card.

Step 6: Batch operations and view switching:

- Table View: Click "Table View" in the top\-right corner to compare multiple models in a more compact list format.

Expected Result:Significantly improves developer efficiency when configuring multiple models in a multi\-model environment.

![image 8](/docs_images/img_en_8.png)

# Operation Logs

## Usage Logs

Usage Logs: Audit and Analyze API Calls

Function Description:Records detailed information for all API calls made through the AllRouter.AI gateway, facilitating auditing and cost analysis.

Steps:

Step 1: In the left sidebar, click "Usage Logs". The page will navigate to the Usage Logs interface, displaying a list of API call records.

Step 2: Set the query time range. Click the date picker and select the Start Date and End Date from the calendar.

Step 3: Enter filter criteria. Input relevant keywords in the fields for Token Name, Model Name, Group, or Request ID.

Step 4: Execute the query. Click the "Search" button, and the list will refresh to show the logs that match your criteria.

Step 5: View log details. Check information such as Time, Token, Model, and Cost in the list. Click the "Details" button at the end of a record to see the full request and response.

![image 9](/docs_images/img_en_9.png)

## Drawing Logs

Function Description:Records the execution details of drawing tasks from models like Midjourney, allowing you to view task status, progress, and generated images.

Steps:

Step 1: In the left sidebar, click "Drawing Logs". The page will display a list of Midjourney Task Records.

Step 2: Search for a specific drawing task. Enter the Task ID and select the time range, then click "Search".

Step 3: View the drawing results. Click on the image thumbnails in the "Result Image" column.

Step 4: Check the task status. If a task fails, the "Failure Reason" column will show the specific error information.

![image 10](/docs_images/img_en_10.png)

## Task Logs

Function Description:Records the lifecycle of all asynchronous tasks in the system, such as batch processing and long\-running tasks.

Steps:

Step 1: In the left sidebar, click "Task Logs". The page will navigate to the Task Records management interface.

Step 2: Filter task records. Enter the Task ID or set a time range, then click "Search".

Step 3: Analyze task duration. Compare the Submission Time and Completion Time to assess processing efficiency.

Step 4: Customize displayed columns. Click the "Column Settings" button at the top\-right of the page and select the fields you want to display.

![image 11](/docs_images/img_en_11.png)

# Wallet

Function Description: Manage your account balance, supporting multiple payment methods and referral rewards.

Steps:

Step 1: Click "Wallet Management" in the left sidebar.

Step 2: Enter the amount to top up in the "Recharge Amount" field \(unit: USD\).

Step 3: Under "Select Payment Method", click the WeChat or Stripe icon.

Step 4: Click the corresponding tier under "Select Recharge Amount", or click "Pay" directly.

Step 5: Scan the QR code or follow the payment gateway instructions to complete the transaction.

Prerequisite:You must have a valid payment method.

Expected Result:After the payment is completed, the Current Balance will update in real time.

![image 12](/docs_images/img_en_12.png)

After completing the payment, go to "Wallet Management" → "Bills" to view the recharge bill records for your account.

![image 13](/docs_images/img_en_13.png)

# Merchant Onboarding

## OAuth Authorization

Feature Description:OAuth Authorization Management enables one\-click connection to mainstream AI service providers. It automatically retrieves and manages authentication credentials, eliminating the need to manually copy API keys, ensuring both security and convenience.

Steps:

Step 1:Click \[Merchant Onboarding\] → \[OAuth Authorization\] in the left sidebar to enter the OAuth authorization management page.

Step 2:Select the AI service provider you want to authorize \(e.g., Codex, Anthropic, Antigravity, Gemini CLI, Kimi, Qwen, etc.\), then click the \[Authorize Now\] button on the corresponding card.

Step 3:Follow the instructions on the redirected third\-party platform to complete account login and authorization.

Step 4:After authorization redirection, if manual submission is required, paste the full callback URL \(including code and state\) into the \[Callback URL\] input field on the corresponding service provider card, then click \[Submit Callback URL\] to complete authentication. \(The current version supports automatic polling of authentication status.\)

Step 5:Wait for the system to automatically retrieve and store the authentication credentials to complete the authorization.

Prerequisites:A valid account with the selected AI service provider, supporting OAuth authorization login Logged into the AllRouter.AI platform

Expected Result:After successful authorization, the status of the corresponding service provider card will change from "Not Authenticated" to "Authenticated."The "Connected Services" count in the top\-right corner will update in real time. You can directly use the AI service within the platform without manually configuring API keys. Authentication credentials are securely stored and can be managed on the \[Credentials\] page.

![image 14](/docs_images/img_en_14.png)

## Authentication Files

Feature Description:Credential File Management centralizes the management of OAuth\-generated credential files, monitors their health status, and allows configuration of model aliases and filtering rules.

Steps:

Step 1:Click \[Merchant Onboarding\] → \[Credential Files\] in the left sidebar to access the credential file management page.

Step 2:At the top of the page, view summary statistics such as total file count, healthy nodes, and anomaly alerts.

Step 3:Use the \[Search by file name or model…\] input box or the \[All Providers\] dropdown filter to quickly locate target credential files.

Step 4:In the file list, check details such as provider, health status, and last active time. Click the corresponding buttons in the \[Actions\] column to perform operations such as edit, configure model aliases/filter rules, refresh status, or delete.

Step 5:After completing the configuration, the system automatically saves the settings and updates the file health status in real time.

![image 15](/docs_images/img_en_15.png)

# Marketing Campaigns

## Invite Reward

Feature Description: The Referral Rewards Program allows you to invite new users to register באמצעות your exclusive referral link. You can earn a share of their compute usage revenue, with support for reward transfer, invitation tracking, and multi\-channel sharing.

Steps:

Step 1: Click \[Marketing Campaigns\] → \[Referral Rewards\] in the left sidebar to enter the referral rewards program page.

Step 2: In the \[Your Exclusive Referral Link\] section, click the \[Copy Link\] button to copy your unique referral link. You can also quickly share it via WeChat, X, or email using the icons below.

Step 3: Share the referral link with new users and guide them to complete registration and their first top\-up through the link.

Step 4: At the top of the page, view summary statistics such as pending rewards, total accumulated rewards, and number of successful referrals.

Step 5: To withdraw rewards, click the \[Transfer to Balance\] button to move pending rewards into your account balance.

Step 6: In the \[Referral Details\] section, view detailed information about invited users, including registration time, status, and contributed rewards.

![image 16](/docs_images/img_en_16.png)

## Redemption Code

Feature Description: Redeem Code Management allows you to enter exclusive redeem codes to activate matrix computing power, increase token balance, or unlock access to advanced models. It also supports redemption history tracking and acquiring codes through multiple channels.

Steps:

Step 1: Click \[Marketing Campaigns\] → \[Redeem Codes\] in the left sidebar to enter the redeem code page.

Step 2: Enter a valid exclusive redeem code in the \[Enter Redeem Code\] input field.

Step 3: Click the \[Redeem Now\] button on the right to submit the redemption request.

Step 4:
In the \[My Redemption Records\] section, view detailed information such as redemption time, code, value/benefits, status, and credit status.

Step 5: In the \[Get More Redeem Codes\] section, you can obtain new redeem codes through methods such as inviting friends, following official accounts, or contributing to the community.

![image 17](/docs_images/img_en_17.png)

# Personal Center

## Personal Settings

Function Description:Manage account bindings, security settings, configure usage limit alerts, and customize interface preferences.

![image 18](/docs_images/img_en_18.png)

### Account Binding

Function Description:Allows you to select the type of social account to bind with your AllRouter.AI account.

![image 19](/docs_images/img_en_19.png)

### Security Settings

Function Description:Allows users to manage system access tokens, passwords, PassKey login, and two\-factor authentication \(2FA\) settings.

![image 20](/docs_images/img_en_20.png)

### Preference Settings

Function Description:Manage interface language and other personal preferences.

![image 21](/docs_images/img_en_21.png)

### Notification Settings

Function Description:Manage notifications, pricing alerts, and privacy\-related settings.

Steps:

Step 1: Click "Personal Settings" in the left sidebar.

Step 2: Click the "Notification Settings" tab \(located in the middle of the page’s tab bar\).

Step 3: Set the trigger amount in the "Usage Alert Threshold" field.

Step 4: Select the notification method \(e.g., email notifications\).

Step 5: Click the "Save Settings" button at the bottom.

Expected Result:When your balance falls below the specified threshold, the system will automatically send an alert through the selected channels.

![image 22](/docs_images/img_en_22.png)

### Price Settings

When a model does not have a set price, it can still be called. Use this option only if you trust the website, as it may incur high costs.

![image 23](/docs_images/img_en_23.png)

### Privacy Settings

When enabled, only "Consumption" and "Error" logs will record your client IP address.

![image 24](/docs_images/img_en_24.png)

# Usage Example

Using AllRouter in Claude Code as an example.

## Step 1: Install Claude Code

Prerequisites:

- You need to have Node.js 18 or a later version installed.
- MacOS users: It is recommended to install Node.js via nvm or Homebrew. Direct package installation is not recommended as it may cause permission issues later.
- Windows users: You also need to install Git for Windows.

Next, open the command\-line interface and install Claude Code.

npm install \-g @anthropic\-ai/claude\-code

Run the following command to verify the installation. If the version number is displayed, the installation was successful.

claude \-\-version

## Step 2: Configure AllRouter

Step 1: Register an Account

Visit the AllRouter platform and click the "Register/Login" button at the top\-right corner. Follow the prompts to complete the account registration and login process.

Step 2: Obtain an API Key

After logging in, go to the Personal Center and click API Keys to create a new API Key.

To start calling the API, you need to create a token on the AllRouter platform first:

- Click "Token Management" in the left sidebar.
- Click the "Add Token" button.
- Set the token name, usage limit, expiration time, and access restrictions.
- Once the token is successfully created, copy the generated Token \(Key\). You can then use it in your application.

![image 25](/docs_images/img_en_25.png)

Copy the API Key information.

![image 26](/docs_images/img_en_26.png)

3. Step 3: Configure Environment Variables

Set environment variables on MacOS, Linux, or Windows \(example shown for manual configuration\).

Supported Systems: MacOS, Linux, and Windows.

Important Notes:

1. The configuration file paths differ between operating systems.
2. Ensure the JSON file format is correct \(no extra or missing characters\) when modifying.

\# Edit or Create the \`settings.json\` File

\# MacOS & Linux is ~/.claude/settings.json

\# Windows is C:\\Users\\<YourUser>\\.claude\\settings.json

\# Add or Modify the env Field in settings.json

\# Be sure to replace "your\_allrouter\_api\_key" in the env field with the actual API Key you obtained from the previous step on AllRouter.AI.

\{

  "env": \{

    "ANTHROPIC\_AUTH\_TOKEN": "your\_allrouter\_api\_key",

    "ANTHROPIC\_BASE\_URL": "https://allrouter.ai/v1",

    "API\_TIMEOUT\_MS": "3000000",

    "CLAUDE\_CODE\_DISABLE\_NONESSENTIAL\_TRAFFIC": 1

  \}

\}

\# Edit or Create the .claude.json File Again

\# On macOS & Linux: ~/.claude.json

\# On Windows: ~/\\.claude.json \(located in the user home directory\)

\# Add the hasCompletedOnboarding Parameter

\{

  "hasCompletedOnboarding": true

\}

Configuration Result:

ANTHROPIC\_AUTH\_TOKEN=Token / API Key

ANTHROPIC\_BASE\_URL=https://allrouter.ai/v1

ANTHROPIC\_MODEL=Token Name

![image 27](/docs_images/img_en_27.png)

## Step 3: Start Using Claude Code

After completing the configuration, navigate to one of your code working directories and run the following command in the terminal:

Tip: If prompted with "Do you want to use this API key", select Yes  to proceed. Your Claude Code environment is now ready to interact with AllRouter.AI models.

![image 28](/docs_images/img_en_28.png)
