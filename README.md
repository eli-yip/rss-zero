# RSS-ZERO

An all-in-zero rss aggregator like [rsshub](https://docs.rsshub.app/), but support more private websites.

## Usage

You can easily start a rss-zero instance with `docker compose`:

```yaml
version: "3.9"
services:
  rss-zero:
    image: "eliyip/rss-zero:latest-slim"
    container_name: rss-zero
    restart: always
    environment:
      - MINIO_ENDPOINT=example.com
      - MINIO_ACCESS_KEY_ID=WbHp********8fYsCR
      - MINIO_SECRET_ACCESS_KEY=Do3MIqH********48sYwDFXJJlr5
      - MINIO_BUCKET_NAME=test
      - MINIO_ASSETS_PREFIX=https://cdn.example.com/abc

      - OPENAI_API_KEY=sk-Og************************0
      - OPENAI_BASE_URL=https://********/v1

      - DB_HOST=rss-zero-db
      - DB_PORT=5432
      - DB_USER=postgres
      - DB_PASSWORD=postgres-password
      - DB_NAME=postgres

      - REDIS_ADDR=rss-zero-redis:6379
      - REDIS_PASSWORD=

      - BARK_URL=

      - ZSXQ_TEST_URL=
    ports:
      - 8080:8080

  rss-zero-db:
    image: "postgres:15.5-apline"
    container_name: rss-zero-db
    restart: always
    environment:
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: postgres-password
      POSTGRES_DB: postgres
    volumes:
      - ./data/db:/var/lib/postgresql/data
    expose:
      - 5432

  rss-zero-redis:
    image: redis:latest
    container_name: rss-zero-redis
    restart: always
    expose:
      - 6379
    volumes:
      - ./data/redis:/data
```

**Remember to fill blank environment variables**.

## Roadmap

I plan to support following websites:

- [x] [Zhishixingqiu](https://zsxq.com/): Need cookies and valid payments
- [ ] [Zhihu](https://www.zhihu.com): No need for cookies, only support answers, articles and pins.  
      **With db and cache, it can get zhihu content more safely and properly**.
- [ ] [Xiaobaotong](https://xiaobot.net): Need `authorization` header and valid payments.

Development progress can be seen in [project milestones](https://git.momoai.me/yezi/rss-zero/milestones).
