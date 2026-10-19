
# Copyright 2021 Google LLC
#
# Licensed under the Apache License, Version 2.0

# --------------------------------------------------
# Build Stage
# --------------------------------------------------
FROM --platform=$BUILDPLATFORM mcr.microsoft.com/dotnet/sdk:9.0.101-noble@sha256:1f13e67d295e02abdfd187c341f887442bad611eda536766172ced401fc8b9fa AS builder

ARG TARGETARCH

# Use the native protoc compiler to avoid
# Grpc.Tools 2.71.0 ARM64 protoc crash.
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
       protobuf-compiler \
    && rm -rf /var/lib/apt/lists/*

# Tell Grpc.Tools to use the system protoc.
ENV PROTOBUF_PROTOC=/usr/bin/protoc

WORKDIR /app

COPY cartservice.csproj .

RUN dotnet restore cartservice.csproj \
    -a $TARGETARCH

COPY . .

RUN dotnet publish cartservice.csproj \
    -p:PublishSingleFile=true \
    -a $TARGETARCH \
    --self-contained true \
    -p:PublishTrimmed=true \
    -p:TrimMode=full \
    -c Release \
    -o /cartservice

# --------------------------------------------------
# Runtime Stage
# --------------------------------------------------
FROM mcr.microsoft.com/dotnet/runtime-deps:9.0.1-noble-chiseled@sha256:6f7466eda39e24efaf7eab2325e15d776a685d13cc93b4ea0cde9ee4f7982210

WORKDIR /app

COPY --from=builder /cartservice .

EXPOSE 7070

ENV DOTNET_EnableDiagnostics=0 \
    ASPNETCORE_HTTP_PORTS=7070

USER 1000

ENTRYPOINT ["/app/cartservice"]
