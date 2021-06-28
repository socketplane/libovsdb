FROM golang:1.16-alpine as base
COPY . /src
WORKDIR /src

FROM base as stress
RUN go install ./cmd/stress

FROM base as print_schema
RUN go install ./cmd/print_schema

FROM base as modelgen
RUN go install ./cmd/modelgen