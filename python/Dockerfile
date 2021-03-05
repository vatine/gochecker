FROM golang:1.16-buster

COPY requirements.txt /

RUN apt-get update && \
    apt-get install -y python3.7 python3-pip && \
    pip3 install -r /requirements.txt

COPY wrapper.py /

ENV GO111MODULE on
ENTRYPOINT [ "/wrapper.py" ]