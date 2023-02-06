# vmagent-config-updater

vmagent-config-updater is a part of prometheus-benchmark.
It is used for generation of -promscrape.config for vmagent in prometheus-benchmark.

See full list of configuration flags by passing `-help` flag to the binary.

### build image 
* cd .../services/vmagent-config-updater
* GOOS=linux GOARCH=amd64 go build  -tags netgo -o bin/vmagent-config-updater-prod
* docker build . -t you_img_tag
* docker push you_img_tag