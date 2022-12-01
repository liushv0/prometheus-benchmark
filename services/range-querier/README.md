### how to build

build  range-querier / 构建range-querier
```shell
cd ./services/range-querier
mkdir bin
GO111MODULE=on CGO_ENABLED=0 GOOS=linux  GOARCH=amd64 go build -o ./bin/
docker build -t YOUR_IMAGE_TAG .
docker push YOUR_IMAGE_TAG
```


then edit `chart/templates/vmalert/deployment.yaml`, replace image name of container range-querier with YOUR_IMAGE_TAG. 
构建完成后编辑 `chart/templates/vmalert/deployment.yaml`, 替换掉其中的range-querier 容器的镜像名
