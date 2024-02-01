const express = require('express');
const app = express();
const { calculateXZSE96 } = require('./encrypt.js');

app.use(express.json());

app.post('/encrypt', (req, res) => {
  const cookieMes = req.body.cookie_mes;
  const apiPath = req.body.api_path;

  if (!cookieMes || !apiPath) {
    console.log('Missing parameters');
    return res.status(400).send('Missing parameters');
  }

  xzse96 = calculateXZSE96(apiPath, cookieMes)
  res.send({ xzse96 });
});

const PORT = 3000;
app.listen(PORT, () => {
  console.log(`Server running on port ${PORT}`);
});
