#!/bin/bash
# Exit immediately if a command exits with a non-zero status.
# Treat unset variables as an error.
# The return value of a pipeline is the status of the last command to exit with a non-zero status.
set -euo pipefail

# --- Pre-flight Checks & Color Definitions ---
#
# Helper for colored output
RED='\033[0;31m'
GREEN='\033[0;32m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Function to check for required commands
check_command() {
  if ! command -v "$1" &> /dev/null; then
    echo -e "${RED}ERROR: Command '$1' is not installed. Please install it and try again.${NC}"
    exit 1
  fi
}

# Function to check that environment variables are not empty
check_vars() {
  for var_name in "$@"; do
    # The :- syntax prevents an "unbound variable" error if the var doesn't exist
    if [[ -z "${!var_name:-}" ]]; then
      echo -e "${RED}ERROR: Environment variable '$var_name' is missing or empty!${NC}"
      echo "This variable must be set either in your '.env' file or as a GitHub Actions secret."
      exit 1
    fi
  done
}

echo -e "${CYAN}--- Running Validator App CI/CD Script ---${NC}"

echo "1. Checking for required tools..."
check_command "docker"
check_command "gcloud"
check_command "gh"
echo -e "${GREEN}All tools are present.${NC}"

# --- Load and Check Environment Variables ---
#
# For local execution, load variables from a .env file if it exists
if [[ -f .env.prod ]]; then
  echo "2. Loading environment variables from .env file..."
  # 'export' makes them available to sub-processes like gcloud and docker
  export $(grep -v '^#' .env.prod | xargs)
else
  echo "2. Skipping .env file load (not found). Assuming variables are set in the environment (e.g., GitHub Actions)."
fi

# Define ALL variables your application needs at runtime
REQUIRED_VARS=(
    "GOOGLE_CLOUD_PROJECT"
    "GOOGLE_CLOUD_VM_REGION"
    "GOOGLE_CLOUD_AR_REPO_NAME"
    "GOOGLE_CLOUD_CR_SERVICE_NAME"
    "FE_HOST"
    "SIGNING_SECRET"
    "GOOGLE_CLOUD_VM_ZONE"
    "GOOGLE_CLOUD_MODLIST_FILE"
    "GOOGLE_CLOUD_VM_NAME"
    "GOOGLE_CLOUD_FIREWALL_NAME"
    "MINECRAFT_SERVER_PORT"
    "MINECRAFT_RCON_PASS"
    "MINECRAFT_RCON_PORT"
    "GITHUB_CLIENT_ID"
    "GITHUB_CLIENT_SECRET"
    "GOOGLE_CLOUD_BUCKET_NAME"
    "SSH_PRIVATE_KEY_BASE64"
    "SSH_VM_USER"
    "SSH_LOG_PATH"
)

echo "3. Verifying all required environment variables are set..."
check_vars "${REQUIRED_VARS[@]}"
echo -e "${GREEN}All required variables are set!${NC}"


# --- Build and Push Docker Image ---
#

## IMPORTANT: EVER SINCE WE MOVED ON TO GO, WE NEED TO PROVIDE `GOOGLE_SERVICE_ACCOUNT_EMAIL`
## AS ENV VAR TO SIGN BLOBS.
export SERVICE_ACCOUNT="main-53fas@${GOOGLE_CLOUD_PROJECT}.iam.gserviceaccount.com"
export IMAGE_NAME="${GOOGLE_CLOUD_VM_REGION}-docker.pkg.dev/${GOOGLE_CLOUD_PROJECT}/${GOOGLE_CLOUD_AR_REPO_NAME}/${GOOGLE_CLOUD_CR_SERVICE_NAME}:latest"
export GOOGLE_CLOUD_SERVICE_ACCOUNT_EMAIL="$SERVICE_ACCOUNT"

# im just lazy

echo -e "${CYAN}--- Configuration ---${NC}"
echo "Service Account: ${SERVICE_ACCOUNT}"
echo "Image Name:      ${IMAGE_NAME}"
echo -e "${CYAN}-------------------${NC}"

gcloud artifacts repositories describe "${GOOGLE_CLOUD_AR_REPO_NAME}" \
  --project="${GOOGLE_CLOUD_PROJECT}" \
  --location="${GOOGLE_CLOUD_VM_REGION}" || \
(
  echo "Repository '${GOOGLE_CLOUD_AR_REPO_NAME}' not found. Creating it..."
  gcloud artifacts repositories create "${GOOGLE_CLOUD_AR_REPO_NAME}" \
    --project="${GOOGLE_CLOUD_PROJECT}" \
    --repository-format=docker \
    --location="${GOOGLE_CLOUD_VM_REGION}" \
    --description="Docker repository for the Validator App" \
    --quiet
)
echo -e "${GREEN}Artifact Registry repository is ready.${NC}"

echo "4. Authenticating with Google Cloud..."
gcloud auth configure-docker "${GOOGLE_CLOUD_VM_REGION}-docker.pkg.dev" --quiet

docker build -t "$IMAGE_NAME" .

echo "6. Pushing image to Google Artifact Registry..."
docker push "$IMAGE_NAME"

ENV_VARS_STRING=""
for var in "${REQUIRED_VARS[@]}"; do
  ENV_VARS_STRING+="${var}=${!var},"
done
# Append the specific Service Account Email var we constructed manually
ENV_VARS_STRING+="GOOGLE_SERVICE_ACCOUNT_EMAIL=${GOOGLE_SERVICE_ACCOUNT_EMAIL}"

# Remove trailing comma
ENV_VARS_STRING=${ENV_VARS_STRING%,}

# --- Deploy to Cloud Run ---
#
echo "7. Deploying to Google Cloud Run..."
gcloud run deploy "$GOOGLE_CLOUD_CR_SERVICE_NAME" \
  --image="$IMAGE_NAME" \
  --platform=managed \
  --service-account="$SERVICE_ACCOUNT" \
  --region="$GOOGLE_CLOUD_VM_REGION" \
  --set-env-vars="$ENV_VARS_STRING" \
  --allow-unauthenticated \

echo -e "${GREEN}Deployment to Cloud Run completed!${NC}"