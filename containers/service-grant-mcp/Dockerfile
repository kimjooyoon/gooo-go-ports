# Build with the context prepared by tools/service_grant_mcp_container.py.
# The complete verified native bundle includes its licenses and build identity.
FROM scratch
COPY runtime/ /opt/gooo/
ENV PATH="" GOPROXY=off GOSUMDB=off GOTELEMETRY=off
USER 65532:65532
WORKDIR /opt/gooo
ENTRYPOINT ["/opt/gooo/gooo-service-grant-mcp"]
CMD []
