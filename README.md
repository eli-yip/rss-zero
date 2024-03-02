# RSS-ZERO

> [!CAUTION]
> This repository is a mirror of rss-zero in my own gitea server.

An all-in-zero rss aggregator like [rsshub](https://docs.rsshub.app/), but support more private websites.

By the way, you can only run the crawlers for Zhishixingqiu or Zhihu if you only want to save content to db.

## Usage

### RSS Server

You can easily start a rss-zero instance with `docker compose`, see `compose.yaml` and `example.env` in `deploy` directory. Rename `example.env` to `.env` file, replace some env variables, then run `docker compose up -d`.

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

Development progress can be seen in [project milestones](https://gitea.momoai.me/yezi/rss-zero/milestones).
