# Smart Nudge Foundation: Architecture Breakdown

## 1. The Core Narrative
**"What is Smart Nudge Foundation?"**
It is a machine-learning-driven recommendation engine designed to optimize facility bookings (or similar user behaviors). It takes raw user activity data, processes it through an ETL pipeline, and uses advanced ML models (like GRU and XGBoost) to generate personalized "nudges" or recommendations for users.

**The Problem I Solved:**
User engagement can be passive. To proactively drive engagement, the system needs to predict what a user might need next based on sequential behavior. Traditional rules-based systems fail to capture temporal patterns.

**The Solution:**
I built an end-to-end ML pipeline. 
1.  **Data Ingestion:** Extracts raw booking data (`facility_bookings_*.csv`).
2.  **ETL & Feature Engineering:** Transforms raw data into sequential features suitable for deep learning.
3.  **Model Training:** Uses a Gated Recurrent Unit (GRU) to capture the *sequence* of user actions over time, and XGBoost for robust classification/ranking of the final nudges.
4.  **Batch Generation:** Outputs a ranked list of recommended actions ("nudges") per user.

## 2. Technical Component Breakdown

### A. The ETL Pipeline (`etl_pipeline.py`)
- **Role:** The backbone of the data processing. Cleans missing values, handles data type conversions, and normalizes time-series data.
- **DevOps/Harness Angle:** ETL pipelines are essentially CI/CD pipelines for data. If you were deploying this in a real enterprise, you would use a tool like Harness or Airflow to orchestrate these ETL steps reliably.

### B. Feature Factory (`feature_factory.py`)
- **Role:** Generates complex features (e.g., "Days since last booking", "Preferred facility type").
- **Why it matters:** Good ML is 80% feature engineering. Separating this from the training logic ensures the features can be re-used across different models (GRU vs XGBoost).

### C. The ML Engine (`train_gru_model.py` / XGBoost)
- **Role:** The brain. The GRU (a type of RNN) is specifically chosen because it handles sequential data (User booked A -> then B -> then C) much better than standard feed-forward networks. XGBoost is likely used as an ensemble or final ranker because it is incredibly fast and interpretable.

## 3. How to Frame this for a "Harness" Interview

*Omkar will care less about the math of a GRU, and more about HOW this code gets to production.*

**Talking Point 1: MLOps vs DevOps**
"Building the GRU model was the easy part. The hard part is the **Outer Loop**—how do we train this model every night reliably? In this project, I realized the need for automated pipelines. Just like Harness automates code delivery, an ML project needs an automated pipeline to re-train the model, validate the new accuracy, and deploy the new weights without breaking the production API."

**Talking Point 2: Handling Failures (The SRE Mindset)**
"When running batch nudge generators (`batch_nudge_generator.py`), if the ETL step fails halfway, we can't afford to output corrupt recommendations. I learned the importance of **Idempotency** and **Graceful Degradation**—if the daily batch fails, the system should serve yesterday's cached nudges rather than crashing the UI."

**Talking Point 3: Infrastructure**
"Right now, this runs via Python scripts. To scale this, I would containerize the training job via Docker, run it as a Kubernetes CronJob, and use a CI/CD platform to manage the deployment lifecycle of the inference API."
