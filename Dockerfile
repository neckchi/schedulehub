FROM ubuntu:latest
LABEL authors="neck.chi"

ENTRYPOINT ["top", "-b"]