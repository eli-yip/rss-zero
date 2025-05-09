# RSS-ZERO

> [!CAUTION]
> This repository is a mirror of rss-zero in my own gitea server.

An all-in-one RSS aggregator that smoothly supports private websites. No headless browser required, most of the code is implemented in pure Go.

Acknowledgment: Thanks to [Soulteary's RSS-Can](https://github.com/soulteary/rss-can). Without that project, I would not have attempted to implement this project.

## Deployment

Check the `compose.yaml` and `config.toml` in the `deploy` folder, modify the configuration to your own values, use `docker compose up -d` to start the container, and the server will run on port `:8080`.

## Usage

### Supported Websites

#### Zhihu

1. `GET /rss/zhihu/answer/{author_id}`
2. `GET /rss/zhihu/article/{author_id}`
3. `GET /rss/zhihu/pin/{author_id}`

`author_id` is Zhihu user's URL token, you can find it by visiting his/her homepage like `https://www.zhihu.com/people/superpeople-wudi`. In this case, `author_id` is `superpeople-wudi`.

Authentication information needs to be input through API, documentation is under construction, you can refer to the code first.

#### GitHub

GitHub's built-in RSS subscription contains tags, but the truly useful part is actually just the Releases:

1. `GET /rss/github/{user}/{repo}`
2. `GET /rss/github/pre/{user}/{repo}`

#### End of Life

End Of Life's built-in RSS generates RSS for all new content, the implementation here filters out the latest and LTS versions.

1. `GET /rss/endoflife/{product_name}`

#### Xiaobot

1. `GET /rss/xiaobot/{paper_id}`

Authentication information needs to be input through API, documentation is under construction, you can refer to the code first.

#### ZSXQ (Knowledge Planet)

1. `GET /rss/zsxq/{group_id}`

Authentication information needs to be input through API, documentation is under construction, you can refer to the code first.

### Export

Supports export of content sources from Zhihu, ZSXQ, and Xiaobot. Documentation is under construction, you can refer to the code first.
