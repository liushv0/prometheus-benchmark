FROM scratch

EXPOSE 8436

ENTRYPOINT ["/vmagent-config-update-prod"]
ARG src_binary=bin/vmagent-config-updater-prod
COPY $src_binary ./vmagent-config-update-prod
