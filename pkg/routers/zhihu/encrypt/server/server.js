const express = require('express');
const app = express();
const { md5, encrypt } = require('./encrypt.js');

app.use(express.json());

app.post('/encrypt', (req, res) => {
  const cookie_mes = req.body.cookie_mes;
  const apiPath = req.body.api_path;

  if (!cookie_mes || !apiPath) {
    console.log('Missing parameters');
    return res.status(400).send('Missing parameters');
  }

  const f = `101_3_3.0+${apiPath}+${cookie_mes}`;
  const xzse96 = '2.0_' + encrypt(md5(f));

  res.send({ xzse96 });
});

const PORT = 3000;
app.listen(PORT, () => {
  console.log(`Server running on port ${PORT}`);
});
