#FROM centurylink/ca-certs
FROM ubuntu
EXPOSE 80
WORKDIR /app
# copy binary into image
COPY movies_backend /app/
COPY .env /app/
#ENTRYPOINT ["./movies_backend"]
