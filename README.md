https://opentelemetry.io/docs/collector/getting-started/#local

## Setup
This should build the OT collector and start it:
```
git clone https://github.com/patsonluk/opentelemetry-collector.git
cd opentelemetry-collector
make install-tools
make otelcorecol
FS_APP_KEY="<FS staging API key>" FS_API_HOST="http://127.0.0.1:9071" ./bin/otelcorecol_* --config ./examples/local/otel-config.yaml
```

`FS_APP_KEY` is required. This is the app/api key generated from FS UI under your account
`FS_API_HOST` is optional. If left blank, then it will send server events to FS staging. Otherwise the example above is for FS local dev.

For local dev env, you would also want to enable server event by editing `$FS_HOME/etc/localdev/featureflags.yaml` add entry `customhouse-enable-send-custom-events: true`

To enable OT HTTP request/response header recording, you might need to enable from FS UI `Settings->Data Capture->Network Data Capture`


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

## Run our modified collector with Jaeger
1. Start jaeger all in one but do NOT bind the OLTP ports (ie 4317, 4318)
```
docker run -d --name jaeger \
  -e COLLECTOR_ZIPKIN_HOST_PORT=:9411 \
  -e COLLECTOR_OTLP_ENABLED=true \
  -p 6831:6831/udp \
  -p 6832:6832/udp \
  -p 5778:5778 \
  -p 16686:16686 \
  -p 14250:14250 \
  -p 14268:14268 \
  -p 14269:14269 \
  -p 9411:9411 \
  jaegertracing/all-in-one:1.39
```
2. Use config `otel-config-with-jaeger.yaml` instead to start the collector, now on top of sending data to FS api (hacked loggingexporter), it should also use the jaeger exporter that export data to the all-in-one docker instance from step 1 . `FS_APP_KEY="<your FS API key>" FS_API_HOST="http://127.0.0.1:9071" ./bin/otelcorecol_* --config ./examples/local/otel-config-with-jaeger.yaml`
