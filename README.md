https://opentelemetry.io/docs/collector/getting-started/#local

### Setup
```
git clone https://github.com/patsonluk/opentelemetry-collector.git
cd opentelemetry-collector
make install-tools
make otelcorecol
FS_APP_KEY="<FS staging API key>" ./bin/otelcorecol_* --config ./examples/local/otel-config.yaml
```

This should build the OT collector and start it

To generate FS sessions and trace events, follow [this](https://github.com/patsonluk/opentelemetry-playground/blob/main/README.md#setup). **BUT**
1. `Run an OT instrumented node js app` remains the same
2. Skip `Compile a test agent exporter`, this OT collector is a REPLACEMENT of such agent exporter
3. `Run an OT instrumented java webapp`, in step 2. Use this shorter command instead `MAVEN_OPTS="-javaagent:../opentelemetry-javaagent.jar" mvn jetty:run-war`

Goto localhost:1234, click on the only button on the page:
1. FS is installed for such webapp, so it records the UI actions.
2. Clicking the button triggers an outbound HTTP request to localhost:8080/test-servlet (the java webapp). Some minor modification is added the the script to inject `traceparent` header with FS ids and send a request
3. The java webapp is instrumented by OT agent. It would produce traces that describe the server activity, such traces would be sent to the modified collector of this project
4. The modified collector would convert the traces into FS server event and POST them. 
5. The console logs of this collector should print out alot of into (in red text)
