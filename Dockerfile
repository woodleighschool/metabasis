# syntax=docker/dockerfile:1

FROM docker.io/library/python:3.12-alpine3.22

ENV DB_PATH="/config/schedules.db"

USER root

COPY requirements.txt pyproject.toml ADOverseas.py /app/
WORKDIR /app

RUN \
    apk add --no-cache \
        sqlite \
    && \
	pip3 install --no-cache-dir -r \
		requirements.txt \
	&& \
	pip3 install --no-cache-dir \
		. \
	&& \
	rm -rf /tmp/*

USER nobody:nogroup

CMD ["ADOverseas"]