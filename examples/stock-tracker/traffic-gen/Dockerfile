FROM mcr.microsoft.com/playwright:v1.57.0-jammy

# Install Bun
ENV BUN_INSTALL=/root/.bun \
    PLAYWRIGHT_BROWSERS_PATH=/ms-playwright
RUN apt-get update && apt-get install -y unzip && \
    curl -fsSL https://bun.sh/install | bash && \
    ln -s /root/.bun/bin/bun /usr/local/bin/bun

WORKDIR /app

# 1) Copy only dependency manifests first (better layer cache)
COPY package.json bun.lock* ./

# 2) Install JS deps
RUN bun install

# 3) Copy the frequently-changing app code last
COPY . .

CMD ["bun", "run", "traffic.ts", "--duration", "0", "--include-chaos"]

