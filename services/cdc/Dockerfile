FROM confluentinc/cp-kafka-connect:7.6.1

RUN confluent-hub install --no-prompt debezium/debezium-connector-postgresql:2.4.2 \
 && confluent-hub install --no-prompt confluentinc/kafka-connect-elasticsearch:14.1.0
