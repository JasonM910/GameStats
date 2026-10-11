FROM sbtscala/scala-sbt:eclipse-temurin-alpine-21_1.x

WORKDIR /app
COPY project/build.properties project/
COPY build.sbt ./
RUN sbt -batch update

COPY src/ src/
RUN sbt -batch test

ENV GAMESTATS_INPUT=/output/partidas.json \
    GAMESTATS_OUTPUT=/output/resultados.json

ENTRYPOINT ["sbt", "-batch", "run"]
