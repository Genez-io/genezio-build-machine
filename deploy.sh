set -e

# if $1 is empty then error out
if [ -z "$1" ]; then
    echo "Please provide an environment [dev/prod]"
    exit 1
fi

docker buildx build --load --platform=linux/amd64 -t genezio-$1-build .

docker tag genezio-$1-build:latest 408878048420.dkr.ecr.us-east-1.amazonaws.com/genezio-$1-build:latest

aws ecr get-login-password --region us-east-1 | docker login --username AWS --password-stdin 408878048420.dkr.ecr.us-east-1.amazonaws.com/genezio-$1-build

docker push 408878048420.dkr.ecr.us-east-1.amazonaws.com/genezio-$1-build:latest

# if $1 = dev, then deploy to dev cluster, if $1 = prod, then deploy to prod cluster
if [ $1 = "dev" ]; then
    aws ecs update-service --region us-east-1 --cluster genezio-development --service genezio-dev-build --force-new-deployment
elif [ $1 = "prod" ]; then
    aws ecs update-service --region us-east-1 --cluster genezio-production --service genezio-prod-build --force-new-deployment
fi
