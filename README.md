# NextInBox Backend

## Prerequisites

1. **Install Go:**

   - Download and install Go from the official website: [https://golang.org/dl/](https://golang.org/dl/)
2. **Install Docker:**

   - Download and install Docker from the official website: [https://www.docker.com/get-started](https://www.docker.com/get-started)
3. **Set up environment variables:**

   - Create a `.env` file in the project root directory with the following content:
     ```env
     SUPABASE_PROJECT_REF=
     SUPABASE_ANON_KEY=
     SUPABASE_SERVICE_ROLE_KEY=
     PORT=8080
     ```

## Initialize the Go Project

1. Initialize the Go module and install dependencies:
   ```bash
   go mod init nextinbox
   go mod tidy
   go get github.com/gorilla/mux github.com/joho/godotenv github.com/lengzuo/supa github.com/rs/cors golang.org/x/time/rate
   ```

## Build and Run Locally

1. Build the application:

   ```bash
   go build -o main .
   ```
2. Run the application:

   ```bash
   ./main
   ```
3. Test the application:

   - Access [http://localhost:8080/health](http://localhost:8080/health) in your browser.
   - Or use a tool like `curl`:
     ```bash
     curl http://localhost:8080/health
     ```
   - You should see a response indicating the status is "ok".
   - Alternatively, use Postman to send a GET request to the same URL and verify the response.

## Example JSON for Using the Go API

When using the Go API, send the following JSON in the request body:

### Example:

```json
{
    "user_key": "nib_user_example_key",
    "service_id": "12345-service",
    "template_id": "template-67890",
    "recipients": [
        {
            "email_address": "user@example.com",
            "name": "Alice"
        }
    ],
    "parameters": {
        "customer_name": "Alice",
        "incentive": "10% off",
        "expiration_date": "2023-12-31"
    }
}
```

### Explanation:

- `user_key`: Identifies the user.
- `service_id`: Specifies the service to use.
- `template_id`: Selects the email template.
- `recipients`: Contains recipient details (email and name).
- `parameters`: Custom parameters to personalize the email.

## Docker Instructions

1. **Build the Docker image:**

   ```bash
   docker build -t nextinbox .
   ```
2. **Push the Docker image to Docker Hub:**

   ```bash
   docker login
   docker push your_dockerhub_username/nextinbox:latest
   ```
3. **Run the Docker container locally:**

   ```bash
   docker run -p 8080:8080 nextinbox
   ```

## Deploying on Koyeb Using Docker Image

1. **Log in to Koyeb:**

   - Visit [Koyeb&#39;s website](https://www.koyeb.com/) and log in to your account.
2. **Create a New App:**

   - In the Koyeb dashboard, click on **Create App**.
3. **Configure App Deployment:**

   - Select **Docker Hub** as the deployment source.
   - Enter your Docker image name, e.g., `your_dockerhub_username/nextinbox:latest`.
4. **Deploy the App:**

   - Click on **Deploy** to start the deployment process.
5. **Access the Application:**

   - Once deployed, Koyeb will provide you with a URL to access your application. Use this URL to test the application.
6. **Test Health Endpoint:**

   - Verify that the application is running by accessing the `/health` endpoint in your browser or using `curl`:
     ```bash
     curl <your-koyeb-app-url>/health
     ```
