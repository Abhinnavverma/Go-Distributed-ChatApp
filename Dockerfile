# 1. Start with a small Linux image that has Go installed
FROM golang:1.25-alpine

# 2. Set the working directory inside the container
WORKDIR /app

# 3. Copy the dependency files first (for caching)
COPY go.mod go.sum ./
RUN go mod download

# 4. Copy the rest of the code
COPY . .

# 5. Build the application named "server"
RUN go build -o server ./cmd/server

# 6. Expose the port (Documentary only, but good practice)
EXPOSE 8080

# 7. The Command to run when the container starts
# We force it to listen on 0.0.0.0 so Docker can reach it
CMD ["./server", "-addr=:8080"]