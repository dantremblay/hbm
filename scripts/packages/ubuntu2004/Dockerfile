FROM ubuntu:20.04

COPY entrypoint.sh /entrypoint.sh
COPY build /usr/local/src/hbm

ENV DAPPER_SOURCE /tmp
ENV DAPPER_OUTPUT dist
ENV SHELL /bin/bash
ENV DEBIAN_FRONTEND=noninteractive

WORKDIR ${DAPPER_SOURCE}

RUN set -x \
	&& apt-get update \
	&& apt-get install -y --no-install-recommends dpkg-dev \
	&& rm -rf /var/lib/apt/lists/*

ENTRYPOINT ["/entrypoint.sh"]
