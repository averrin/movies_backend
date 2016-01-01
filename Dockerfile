FROM centurylink/ca-certs
EXPOSE 80
WORKDIR /app
# copy binary into image
COPY movies_backend /app/
ENTRYPOINT ["./movies_backend"]
