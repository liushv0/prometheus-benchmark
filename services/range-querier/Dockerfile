FROM scratch

EXPOSE 8436

ENTRYPOINT ["/range-querier"]
ARG src_binary=bin/range-querier
COPY $src_binary ./range-querier
