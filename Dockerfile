FROM golang:1.21.3-alpine AS build-env
RUN apk add --no-cache bash
RUN apk add make git bash build-base libc-dev binutils-gold
ENV GOPATH=/go
ENV PATH="/go/bin:${PATH}"

# move everything to a temp directory then move everything in the temp directory except the grip directory to grip-endpoints, then delete the temp directory

ADD ./grip /go/src/github.com/bmeg/grip
RUN cd /go/src/github.com/bmeg/grip && make install

ADD ./ /go/src/github.com/bmeg/grip-endpoints
RUN cd /go/src/github.com/bmeg/grip-endpoints && go build -trimpath --buildmode=plugin ./graphql_gen3 && go build -trimpath --buildmode=plugin ./gen3_writer && go build -trimpath --buildmode=plugin ./graphql_peregrine

RUN cp /go/src/github.com/bmeg/grip-endpoints/graphql_gen3.so /
RUN cp /go/src/github.com/bmeg/grip-endpoints/gen3_writer.so /
RUN cp /go/src/github.com/bmeg/grip-endpoints/graphql_peregrine.so /
RUN cp /go/src/github.com/bmeg/grip-endpoints/graphql_peregrine/mongo.yml /


# final stage
FROM alpine
WORKDIR /data
VOLUME /data
ENV PATH="/app:${PATH}"
COPY --from=build-env /graphql_gen3.so /data/
COPY --from=build-env /gen3_writer.so /data/
COPY --from=build-env /graphql_peregrine.so /data/

COPY --from=build-env /mongo.yml /data/
COPY --from=build-env /go/bin/grip /app/