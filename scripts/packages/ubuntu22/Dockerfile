FROM ubuntu:22.04

COPY entrypoint.sh /entrypoint.sh
COPY build /usr/local/src/hbm

ENV SHELL=/bin/bash
ENV DEBIAN_FRONTEND=noninteractive

WORKDIR /tmp

RUN apt-get update \
	&& apt-get install -y --no-install-recommends dpkg-dev fakeroot gzip \
	&& apt-get clean \
	&& rm -rf /var/lib/apt/lists/*

ENTRYPOINT ["/entrypoint.sh"]
