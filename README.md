# RSS-ZERO

> [!CAUTION]
> This repository is a mirror of rss-zero in my own gitea server.

An all-in-zero rss aggregator like [rsshub](https://docs.rsshub.app/), but smoothly support some private websites.

## Usage

### RSS Server

You can easily start a rss-zero instance with `docker compose`, see `compose.yaml` and `example.env` in `deploy` directory. After renamming `example.env` to `.env` file and replace some env variables, you can run with command `docker compose up -d`. Then you can access rss-zero at port `8080`.

#### Supported websites

[Zhihu](https://zhihu.com):

- `GET /rss/zhihu/answer/{author_id}`: This router is for zhihu answer.
- `GET /rss/zhihu/article/{author_id}`: This router is for zhihu article.
- `GET /rss/zhihu/pin/{author_id}`: This router is for zhihu pins.

`author_id` is zhihu user's url token, you can find it by visiting his/her homepage like `https://www.zhihu.com/people/superpeople-wudi`. In this case, `author_id` is `superpeople-wudi`.

[Zhishixingqiu](https://zsxq.com):

- `GET /rss/zsxq/{group_id}`: This router is for zsxq group.
- `POST /api/v1/cookie/zsxq`: This router is for zsxq cookie update.

You can update zsxq cookie by posting a request to above router like:

```json
{
    "cookie": "zsxq_access_token=26964939-EC62-xxxx-1BBE-31376A9FF31C_xxxxxBD88D7"
}
```

`access_token` can be found in Chrome developer tool->networks.

[Xiaobot](https://xiaobot.net):

- `GET /rss/xiaobot/{paper_id}`: This router is for xiaobot paper.
- `POST /api/v1/cookie/xiaobot`: This router is for xiaobot token update.

You can update xiaobot access token by posting a request to abobe router like:

```json
{
    "token": "xxxx|xxxxxxxxx"
}
```

Access token can be found by the same way as zsxq.

[End of Life](https://endoflife.date):

- `GET /rss/endoflife/{product_name}`: This router is for EndOfLife product.

This router will only fetch latest and LTS release of specified product.

#### Export feature

The server also supports zhihu, zsxq and xiaobot export. This part of docs is still under construction, you can refenrence code first.

### Single crawler

Docs of this part is under porgress.

#### Preparation

Before start, you should make sure a postgres database and redis is ready. You can download `.env` file from `master` branch and fill in values. Then you should set your env variables by a command like `export $(xargs < .env)`.

#### Zhishixingqiu crawler

After preparaion, you can download crawler programs from release page.

For zhisihxingqiu crawler, you should first set group id in database and set cookie in redis. Then you can run it.

#### Zhihu crawler

Before start, you should run a server for zhihu encrypted signs:

```bash
docker run -d -p 3000:3000 eliyip/zhihu-encrypt:1.0.2
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

I plan to support following websites:

- [x] [Zhishixingqiu](https://zsxq.com/): Need cookies and valid payments
- [x] [Zhihu](https://www.zhihu.com): No need for cookies, only support answers, articles and pins.  
      **With db and cache, it can get zhihu content more safely and properly**.
- [x] [Xiaobaotong](https://xiaobot.net): Need `authorization` header and valid payments.
- [x] [End of Life](https://endoflife.date)
- [ ] [Weibo](https://weibo.com): Need cookies.

Development progress can be seen in [project milestones](https://gitea.momoai.me/yezi/rss-zero/milestones).
