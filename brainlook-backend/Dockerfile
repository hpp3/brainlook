# Start from a Debian-based image with the latest version of Go installed
FROM golang:latest

# Set the working directory in the container
WORKDIR /usr/src/app

# Copy the local package files to the container's workspace
COPY . .

# Build the Go app
RUN go build -v -o brainlook-backend

# Expose port 8080 to the outside world
EXPOSE 8080

# Run the executable
CMD ["./brainlook-backend"]
