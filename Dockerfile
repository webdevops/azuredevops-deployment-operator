FROM golang:1.15 as build

WORKDIR /go/src/github.com/webdevops/azuredevops-deployment-operator

# Get deps (cached)
COPY ./go.mod /go/src/github.com/webdevops/azuredevops-deployment-operator
COPY ./go.sum /go/src/github.com/webdevops/azuredevops-deployment-operator
RUN go mod download

# Compile
COPY ./ /go/src/github.com/webdevops/azuredevops-deployment-operator
RUN make test
RUN make lint
RUN make build
RUN ./azuredevops-deployment-operator --help

#############################################
# FINAL IMAGE
#############################################
FROM gcr.io/distroless/static

ENV LOG_JSON=1

COPY --from=build /go/src/github.com/webdevops/azuredevops-deployment-operator/azuredevops-deployment-operator /
USER 1000
ENTRYPOINT ["/azuredevops-deployment-operator"]
