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

Evetually we will have a proper instrumented front-end page and instrumented backend that send traces to this collector.

For the moment, if we want to quickly test FS sessions and trace events, follow [this](https://github.com/patsonluk/opentelemetry-playground/blob/main/README.md#setup). **BUT**
1. `Run an OT instrumented node js app` remains the same
2. Skip `Compile a test agent exporter`, this OT collector is a REPLACEMENT of such agent exporter
3. `Run an OT instrumented java webapp`, in step 2. Use this shorter command instead `MAVEN_OPTS="-javaagent:../opentelemetry-javaagent.jar" mvn jetty:run-war`

Goto localhost:1234, click on the only button on the page, the flow will be:
1. FS is installed for such webapp, so it records the UI actions.
2. Clicking the button triggers an outbound HTTP request to localhost:8080/test-servlet (the java webapp). Some minor modification is added to the js to manually inject `traceparent` header with FS ids and send a request
3. The java webapp is instrumented by OT agent. It should produce traces that describe the server activities, such traces would be sent to the modified collector of this project
4. The modified collector (originally just a logging exporter) would convert the traces into FS server event and POST them. 
5. The console logs of this collector should print out alot of info (in red text). The corresponding FS trace URL
![image](https://user-images.githubusercontent.com/2895902/201805194-cf75a0f8-b1c0-4cb3-abb0-981c3d062329.png)
