FROM mcr.microsoft.com/playwright:v1.57.0-jammy

ENV PLAYWRIGHT_BROWSERS_PATH=/ms-playwright

WORKDIR /app

# 1) Copy dependency manifest for caching
COPY package.json package-lock.json* ./

# 2) Install dependencies (npm comes with the Playwright base image)
RUN npm install

# 3) Copy application code
COPY . .

# Default: run forever with chaos on (duration 0 falls back to default loop duration)
CMD ["npm", "run", "traffic", "--", "--duration", "0", "--include-chaos"]

