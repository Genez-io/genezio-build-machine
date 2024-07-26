set -e

# if $1 is empty then error out
if [ -z "$1" ]; then
    echo "Please provide an environment [dev/prod]"
    exit 1
fi

pwd 
docker buildx build --load --platform linux/amd64 -f Dockerfile.dev  --no-cache -t genezio-build-$1 .

docker tag genezio-build-$1:latest harbor-registry.prod.cluster.genez.io/genezio/genezio-build-$1:latest

aws ecr get-login-password --region us-east-1 | docker login --username AWS --password-stdin 408878048420.dkr.ecr.us-east-1.amazonaws.com/genezio-build-$1

docker push 408878048420.dkr.ecr.us-east-1.amazonaws.com/genezio-build-$1:latest
