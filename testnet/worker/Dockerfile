FROM golang:latest 

RUN go get github.com/dexm-coin/dexmd

ADD . /go/src/github.com/dexm-coin/dexmd
WORKDIR /go/src/github.com/dexm-coin/dexmd
RUN go build -o dexmd
RUN cp dexmd testnet/worker/dexmd

RUN cp dexmd testnet/worker/dexmd
WORKDIR /go/src/github.com/dexm-coin/dexmd/testnet/worker

RUN apt-get update && apt-get install -y python-pip
RUN pip install requests 

CMD ["python", "app.py"]