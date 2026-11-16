FROM sbtscala/scala-sbt:eclipse-temurin-alpine-21_1.x AS build

WORKDIR /app
COPY project/build.properties project/
COPY build.sbt ./
RUN sbt -batch update

COPY src/ src/
RUN sbt -batch test stageRuntime

FROM eclipse-temurin:21-jre-alpine
WORKDIR /app
COPY --from=build /app/target/runtime/ /app/lib/

ENV GAMESTATS_INPUT=/output/partidas.json \
    GAMESTATS_OUTPUT=/output/resultados.json

ENTRYPOINT ["java", "-cp", "/app/lib/*", "Main"]
