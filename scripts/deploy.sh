set -e

# if $1 is empty then error out
if [ -z "$1" ]; then
    echo "Please provide an environment [dev/prod]"
    exit 1
fi

# Print working directory
echo "Current directory: $(pwd)"

# Build the Docker image
echo "Building Docker image..."
docker buildx build --load --platform linux/amd64 -f Dockerfile.dev --no-cache -t genezio-build-$1:latest .

# Verify the Docker image exists locally
if [[ "$(docker images -q genezio-build-$1:latest 2> /dev/null)" == "" ]]; then
  echo "Docker image genezio-build-$1:latest was not found locally. Build failed."
  exit 1
fi

# Tag the Docker image
echo "Tagging Docker image..."
docker tag genezio-build-$1:latest 408878048420.dkr.ecr.us-east-1.amazonaws.com/genezio-build-$1:latest

# Log in to AWS ECR
echo "Logging into AWS ECR..."
aws ecr get-login-password --region us-east-1 | docker login --username AWS --password-stdin 408878048420.dkr.ecr.us-east-1.amazonaws.com

# Push the Docker image to ECR
echo "Pushing Docker image to ECR..."
docker push 408878048420.dkr.ecr.us-east-1.amazonaws.com/genezio-build-$1:latest

echo "Docker image pushed successfully."
