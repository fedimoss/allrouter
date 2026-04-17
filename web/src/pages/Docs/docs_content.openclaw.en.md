This manual focuses on the full end-to-end process of connecting AllRouter.AI to Feishu Miaoda OpenClaw. It is intended to help users complete deployment quickly, establish a stable integration between AllRouter.AI and Feishu Miaoda OpenClaw, reduce integration overhead, and improve delivery efficiency.

# I. Enable AllRouter

## 1. Register an AllRouter account and sign in

Visit [https://allrouter.ai/](https://allrouter.ai/), click **Login** -> **Register Account** at the top, enter your username and password in the pop-up window, and then click **Register Now** to complete account registration.

Then sign in with the same account and click **Log In Now** to access AllRouter.AI.

![image 0](/docs_images/openclaw_en_00.png)

![image 1](/docs_images/openclaw_en_01.png)

## 2. Find and choose a model

Function description: **Model Square** is the core model catalog of AllRouter.AI. It shows the AI models supported by the platform and provides transparent, real-time pricing information, so you can evaluate usage cost based on the official multiplier or your actual post-recharge cost.

![image 2](/docs_images/openclaw_en_02.png)

1. Step 1: In the top navigation bar, click **Model Square**. The model list opens in card view by default and shows each model ID with the corresponding 1M Tokens price.
2. Step 2: Understand the two billing modes.

- Pay-as-you-go (Tokens): For text generation models, billing is based on input, output, and cached-read token usage.
- Pay-per-request (Requests): For drawing, task, and other specific models, a fixed fee is charged per request.

3. Step 3: Switch the pricing display mode as needed.

- Recharge price display: When enabled, the system converts pricing based on your recharge ratio and shows the actual deducted amount.
- Rate display: When enabled, each model card shows the billing multiplier relative to the official price.

4. Step 4: Use the sidebar to locate models precisely. Click a specific vendor icon under **Provider**, or select **Pay-as-you-go** under **Billing Type**.
5. Step 5: Search for and copy the model ID. Enter keywords in the top search box, then click the **Copy** icon in the upper-right corner of the target card.
6. Step 6: Use bulk operations and switch views.

- Table view: Click **Table View** in the upper-right corner to compare multiple models in a denser list layout.
- Expected result: This significantly improves configuration efficiency when working with multiple models.

## 3. Create a token and get `Base_URL`, `APIKey`, and `ModelName`

To start calling the API, you need to create a token first:

1. Step 1: Click **Token Management** in the left sidebar.
2. Step 2: Click **Add Token**.
3. Step 3: Set the token name, token group, quota limit, expiration time, and access restrictions.

![image 3](/docs_images/openclaw_en_03.png)

![image 4](/docs_images/openclaw_en_04.png)

4. Step 4: After the token is created, copy the generated token key for use in your application.
5. Step 5: Copy and save the following values:

`base_url=https://allrouter.ai/v1` (all models under this token group can use this base URL)

`api_key=sk-xxxxxxxxxxxxxxxxxxxxxxxxxx`

`ModelName (modelID)=gpt-5.4` (can be copied directly from Model Square)

![image 5](/docs_images/openclaw_en_05.png)

![image 6](/docs_images/openclaw_en_06.png)

![image 7](/docs_images/openclaw_en_07.png)

# II. Build the Feishu Miaoda OpenClaw application

## 1. Sign in to the Feishu Miaoda developer platform

Visit the official Feishu Miaoda developer platform at [https://miaoda.feishu.cn/](https://miaoda.feishu.cn/) and click **Go to Development**.

![image 8](/docs_images/openclaw_en_08.png)

## 2. Deploy OpenClaw with one click

![image 9](/docs_images/openclaw_en_09.png)

## 3. Start creating

![image 10](/docs_images/openclaw_en_10.png)

## 4. Click "Continue"

![image 11](/docs_images/openclaw_en_11.png)

## 5. Set a custom name for the OpenClaw assistant

For example: "Work Assistant", "Personal Assistant", and so on.

![image 12](/docs_images/openclaw_en_12.png)

## 6. Wait for creation to complete

Wait until OpenClaw finishes creating the application.

![image 13](/docs_images/openclaw_en_13.png)

# III. Configure AllRouter parameters in Feishu OpenClaw

## 1. Open the Feishu OpenClaw admin panel

From the previous step, open the Lobster application and enter the Feishu OpenClaw management backend.

![image 14](/docs_images/openclaw_en_14.png)

![image 15](/docs_images/openclaw_en_15.png)

## 2. Configure `base_url`, `api_key`, and `modelID`

Switch to the **Settings** page. Under available models, click **Custom Model**, then fill in the `base_url`, `api_key`, and `modelID` values from Section I in the pop-up dialog.

- Provider: The provider name of the large model service.
- Endpoint: The value of `base_url`.
- API Key: The value of `api_key`.
- Model ID: The value of `ModelName`.

![image 16](/docs_images/openclaw_en_16.png)

The completed configuration looks like this:

![image 17](/docs_images/openclaw_en_17.png)

![image 18](/docs_images/openclaw_en_18.png)

## 3. Switch the model

![image 19](/docs_images/openclaw_en_19.png)

# IV. Test the result

## 1. Open the Feishu agent

![image 20](/docs_images/openclaw_en_20.png)

## 2. Launch Feishu

![image 21](/docs_images/openclaw_en_21.png)

## 3. Run a conversation test

On the Feishu page, send messages to the OpenClaw assistant for testing, such as "Hello", "What can you do?", or "What capabilities do you have?".

![image 22](/docs_images/openclaw_en_22.png)
