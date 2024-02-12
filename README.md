# RSS-ZERO

An all-in-zero rss aggregator like [rsshub](https://docs.rsshub.app/), but support more private websites.

By the way, you can only run the crawlers for Zhishixingqiu or Zhihu if you only want to save content to db.

## Usage

### RSS Server

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

      - BARK_URL=https://api.day.app/xGRiymXNCxxxxxx5fY

      - ZSXQ_TEST_URL="https://api.zsxq.com/v2/groups/${your_group_id}/topics?scope=all&count=20"
      - XIAOBOT_TEST_URL="https://api.xiaobot.net/paper/subscribed"

      - ZHIHU_ENCRYPTION_URL="https://rss-zero-zhihu:3000/encrypt"

      - SERVER_URL="https://rss-zero.example.com"
      - INTERNAL_SERVER_URL="http://rss-zero:8080"
    ports:
      - 8080:8080

  rss-zero-zhihu:
    image: hub.momoai.me/rss-zhihu-encrypt:1.0.0
    container_name: rss-zero-zhihu
    restart: always
    expose:
      - 3000
    networks:
      - traefik-network

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

### Single crawler

#### Preparation

Before start, you should make sure a postgres database and redis is ready. You can download `.env` file from `master` branch and fill in values. Then you should set your env variables by a command like `export $(xargs < .env)`.

#### Zhishixingqiu crawler

After preparaion, you can download crawler programs from release page.

For zhisihxingqiu crawler, you should first set group id in database and set cookie in redis. Then you can run it.

#### Zhihu crawler

Before start, you should run a server for zhihu encrypted signs:

```bash
docker run -d -p 3000:3000 eliyip/zhihu-encrypt:1.0
```

For zhihu crawler, you can run with several args:

- `--answer`: crawl answers of a selected user.
- `--article`: crawl articles of a selected user.
- `--pin`: crawl pins(memos) of a selected user.
- `--user`: set username to crawl. It is called `url_token` in zhihu api. You can find this by `/people` route in zhihu. For example, `canglimo` is the username of `https://www.zhihu.com/people/canglimo`.

By default nothing will be crawl.

For example, if I want to crawl all answers of zhihu user `canglimo`, I should use the following command:

```bash
./zhihu-crawler --user=canglimo --answer
```

## Roadmap

I plan to support following websites, both crawler and rss:

- [x] [Zhishixingqiu](https://zsxq.com/): Need cookies and valid payments
- [x] [Zhihu](https://www.zhihu.com): No need for cookies, only support answers, articles and pins.  
      **With db and cache, it can get zhihu content more safely and properly**.
- [x] [Xiaobaotong](https://xiaobot.net): Need `authorization` header and valid payments.

Development progress can be seen in [project milestones](https://git.momoai.me/yezi/rss-zero/milestones).
