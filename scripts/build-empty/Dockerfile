FROM node:20.15.0-alpine3.20

RUN npm i -g genezio
RUN apk add unzip
RUN apk add git
WORKDIR /app
COPY . .
ENV GENEZIO_API_BASE_URL=https://api.genez.io
RUN npm install
