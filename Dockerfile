FROM golang:1.22.5-alpine AS build-env
RUN apk add --no-cache bash
RUN apk add make git bash build-base libc-dev binutils-gold
ENV GOPATH=/go
ENV PATH="/go/bin:${PATH}"

ADD ./ /go/src/github.com/bmeg/grip-endpoints

RUN go install github.com/bmeg/grip@v0.0.0-20240715185325-a23632989c43
RUN go build  --buildmode=plugin ./graphql_gen3
RUN go build  --buildmode=plugin ./gen3_writer

RUN cp /go/src/github.com/bmeg/grip/mongo.yml /


#FROM alpine
#WORKDIR /data
#VOLUME /data
#ENV PATH="/app:${PATH}"
#COPY --from=build-env /graphql_gen3.so /data/
#COPY --from=build-env /gen3_writer.so /data/
#COPY --from=build-env /graphql_peregrine.so /data/
#COPY --from=build-env /schema.json /data/
#COPY --from=build-env /mongo.yml /data/
#COPY --from=build-env /go/bin/grip /app/
