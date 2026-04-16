# AtlasPay Cloud Deployment Guide

This guide shows how to deploy AtlasPay to a cloud environment so the Saga Orchestrator and dashboard can be demonstrated from a public URL.

## 🚀 5-Minute Deployment (Free Tier)

We use **Render.com** because it supports Docker and has a great free tier for DB/Redis.

### 1. Push to GitHub
If you haven't already, push this codebase to a private/public GitHub repository.

### 2. Connect to Render
1. Go to [Render.com](https://render.com) and sign in with GitHub.
2. Click **New +** > **Blueprint**.
3. Select your `AtlasPay` repository.
4. Render will automatically detect `render.yaml`. 
5. It will prompt you to create:
   - **atlaspay-api**: The Go backend.
   - **atlaspay-db**: Managed PostgreSQL.
   - **atlaspay-redis**: Managed Redis.
6. Click **Apply**.

### 3. Finalize Frontend
Since `web/index.html` is a single file, you have two great options to make it feel like a "Native App":

**Option A: Vercel (Recommended for WOW factor)**
1. Drag and drop the `web/` folder to [Vercel](https://vercel.com/import/general).
2. It will give you a stunning URL like `atlaspay-dashboard.vercel.app`.
3. Open this URL on your phone, then "Add to Home Screen". It will now look and behave like a high-end fintech app.

**Option B: GitHub Pages**
1. In your repo settings, enable GitHub Pages for the `/web` folder.
2. Your URL will be `yourusername.github.io/AtlasPay/web`.

### 4. Wire them together
Once the backend is live, it will give you a URL (e.g., `https://atlaspay-api.onrender.com`).
1. Open your live dashboard URL.
2. In the **"Edge API Endpoint"** field, paste your Render URL.
3. Click **"Connect to Cluster"**.
4. The system will now be live-synced globally!

## 📱 Mobile Demonstration Script
For a quick mobile walkthrough:
1. Pull up the dashboard on your phone.
2. Explain: *"This is a distributed system monitor I built to visualize the Saga pattern. It's running live on a distributed cluster with Redis for state and Postgres for persistence."*
3. Click **"Initialize Cluster Context"**.
4. Run a **"Simulate Success"** to show atomic consistency.
5. Run a **"Simulate Conflict"** to show the system automatically triggering compensating transactions to prevent data corruption.
6. Point to the **Consistency Star (★)**: *"Notice how the orchestrator ensures eventual consistency even when individual nodes fail."*
