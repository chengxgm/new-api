#!/bin/bash
USERNAME="chengxgm"
IMAGE_NAME="new-api"

TAG="latest"

echo "登录到DockerHub..."
docker login -u $USERNAME

echo "构建Docker镜像..."
docker build -t $USERNAME/$IMAGE_NAME:$TAG .

echo "推送镜像到DockerHub..."
docker push $USERNAME/$IMAGE_NAME:$TAG

echo "完成！"
echo $TAG