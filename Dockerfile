FROM golang:1.21.3-alpine AS build-env
RUN apk add --no-cache bash
RUN apk add make git bash build-base libc-dev binutils-gold
ENV GOPATH=/go
ENV PATH="/go/bin:${PATH}"

ADD ./ /go/src/github.com/bmeg/grip

# Move plugin directories into grip subdirectory to ensure dependencies and versions of plugin and grip match
RUN cd /go/src/github.com/bmeg/grip/grip && make install
RUN mv /go/src/github.com/bmeg/grip/graphql_gen3  /go/src/github.com/bmeg/grip/grip/graphql_gen3
RUN mv /go/src/github.com/bmeg/grip/gen3_writer  /go/src/github.com/bmeg/grip/grip/gen3_writer
RUN mv /go/src/github.com/bmeg/grip/graphql_peregrine  /go/src/github.com/bmeg/grip/grip/graphql_peregrine

RUN cd /go/src/github.com/bmeg/grip/grip && go build -trimpath --buildmode=plugin ./graphql_gen3
RUN cd /go/src/github.com/bmeg/grip/grip && go build -trimpath --buildmode=plugin ./gen3_writer
RUN cd /go/src/github.com/bmeg/grip/grip && go build -trimpath --buildmode=plugin ./graphql_peregrine

RUN cp /go/src/github.com/bmeg/grip/grip/graphql_gen3.so /
RUN cp /go/src/github.com/bmeg/grip/grip/gen3_writer.so /
RUN cp /go/src/github.com/bmeg/grip/grip/graphql_peregrine.so /
RUN cp /go/src/github.com/bmeg/grip/mongo.yml /
RUN cp /go/src/github.com/bmeg/grip/schema.json /


FROM alpine
WORKDIR /data
VOLUME /data
ENV PATH="/app:${PATH}"
COPY --from=build-env /graphql_gen3.so /data/
COPY --from=build-env /gen3_writer.so /data/
COPY --from=build-env /graphql_peregrine.so /data/
COPY --from=build-env /schema.json /data/
COPY --from=build-env /mongo.yml /data/
COPY --from=build-env /go/bin/grip /app/

