FROM confluentinc/cp-kafka-connect:7.6.1

USER root
RUN mkdir -p /usr/share/confluent-hub-components \
 && curl -fsSL https://repo1.maven.org/maven2/io/debezium/debezium-connector-postgres/2.4.2.Final/debezium-connector-postgres-2.4.2.Final-plugin.tar.gz \
    | tar -xz -C /usr/share/confluent-hub-components
USER appuser

RUN confluent-hub install --no-prompt confluentinc/kafka-connect-elasticsearch:14.1.0

COPY --chown=1000:1000 connectors/ /connectors/
RUN chmod 644 /connectors/*
