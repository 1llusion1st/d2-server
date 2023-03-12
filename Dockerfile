FROM golang:1.19

RUN apt update
RUN apt install -y curl
RUN curl -sL https://deb.nodesource.com/setup_16.x -o /tmp/nodesource_setup.sh

RUN chmod +x /tmp/nodesource_setup.sh
RUN /tmp/nodesource_setup.sh

RUN apt update

RUN apt install -y nodejs
RUN node -v
RUN npm -v

RUN go install oss.terrastruct.com/d2@latest
RUN npx playwright install-deps
RUN echo 'x -> y' | d2 - /tmp/test.png
RUN ls -lh /tmp/test.png
RUN rm /tmp/test.png


WORKDIR /src
COPY go.mod go.sum /src/
RUN go mod download
COPY main.go /src/

RUN go mod tidy

RUN go build -o /app main.go

EXPOSE 8000

ENTRYPOINT ["/app"]

