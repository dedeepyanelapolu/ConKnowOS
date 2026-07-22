FROM python:3.14-slim

WORKDIR /app

COPY . /app

EXPOSE 8080

CMD ["python", "kernel/cmd/main.py"]
