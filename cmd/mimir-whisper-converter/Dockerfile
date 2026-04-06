# build image from top of repo with with: docker build . -f cmd/mimir-whisper-converter/Dockerfile
FROM python:3.13-bullseye@sha256:f58f33e0563f2ba81c7afe6259cd912f0c33413da93c75cc3a70a941c17afa8c AS build

# Graphite whisper tools
RUN apt update && apt -y install git \
  && git clone https://github.com/graphite-project/whisper.git /whisper

# mimirtool (for uploading blocks)
RUN wget https://github.com/grafana/mimir/releases/download/mimir-2.10.3/mimirtool_2.10.3_amd64.deb \
  && dpkg -i mimirtool_2.10.3_amd64.deb

# aws (for working with S3)
RUN curl "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "awscliv2.zip" \
  && unzip awscliv2.zip \
  && ./aws/install

# thanos (for doing bucket rewriting)
RUN wget https://github.com/thanos-io/thanos/releases/download/v0.32.5/thanos-0.32.5.linux-amd64.tar.gz \
  && tar xfvz thanos-0.32.5.linux-amd64.tar.gz \
  && cp thanos-0.32.5.linux-amd64/thanos /usr/local/bin/

WORKDIR /whisper

RUN python ./setup.py build

FROM python:3.13-bullseye@sha256:f58f33e0563f2ba81c7afe6259cd912f0c33413da93c75cc3a70a941c17afa8c

RUN addgroup --gid 1000 app \
  && adduser --uid 1000 --home /app --gid 1000 --system app \
  && apt update && apt -y --no-install-recommends install prometheus vim groff

RUN apt install -y nano less jq

COPY --from=build /whisper/build /app/opt/whisper
COPY --from=build /usr/local/bin/aws /app/bin/
COPY --from=build /usr/local/bin/thanos /app/bin/
COPY --from=build /usr/local/bin/mimirtool /app/bin/

WORKDIR /app
USER app

COPY dist/mimir-whisper-converter /app/mimir-whisper-converter
COPY scripts/whisper /app/bin
COPY scripts/block-rewriting /app/bin

ENV PYTHONPATH=/app/opt/whisper/lib
ENV PATH=$PATH:/app/opt/whisper/scripts-3.10

CMD ["tail", "-f", "/dev/null"]
