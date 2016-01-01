FROM centurylink/ca-certs
EXPOSE 3001
WORKDIR /app
# copy binary into image
COPY movies_backend /app/
ENTRYPOINT ["./movies_backend"]
