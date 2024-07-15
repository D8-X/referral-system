# Referral System
When trading with a __referral-code__, traders get a kick-back on the trading fees that they have paid, and __referrers__ (the creator of the code) also earn a share of these fees.
The __whitelabelling partner__ (the entity that runs the front-end) can determine the share of trading fees that the referrer obtains and can make this a function of their token holdings.

For example, whitelabelling partner "Pudgy Cow" can set the referral kick-back to 1% if the referrer holds more than 100 Pudgy tokens, and to 5% if they hold more than 1000 Pudgy tokens. The referrer can choose how much
of the 5% (or so) kick-back the trader will receive and they get to keep. Anyone can be a referrer. Whitelabelling partners can set the token address and amount in the back-end configuration.

Alternatively, whitelabelling partners can elect addresses that become an __agency__. Agencies can create referral-codes like a regular referrer and additionally they can elect a partner that then also becomes an agency.
Agencies are not subject to the token configuration. 

For example, whitelabelling partner Pudgy Cow selects marketing agency "Alpha Pots" as an agency and assigns a kick-back of 50%. Alpha Pots work with an influencer to which they hand-off 25% of the 50% kick-back, and they also have an affiliate
agency that they elect as partner with a certain percentage of their 50%. The influencer creates a code which distributes 15% back to the trader. Hence the trader that uses the code receives 50% * 25% * 20% =2.5% fee reduction, the influencer 
receives 50% * 25% * (1-20%) = 10% of the trader's fees, Alpha Pots get 50% * (1-25%)=37.5% and pudgy cow keeps 50% of the fees (checking: 2.5% + 10%+ 37.5% +50%=100%). Alpha Pots' affiliate agency can also elect a partner or
create codes and we can imagine a similar story.

Payments are performed automatically as scheduled by the whitelabelling partner. Configuration of the referral system at hand is detailed in the
[D8X CLI repository](https://github.com/D8-X/d8x-cli/blob/main/README_CONFIG.md).

# API

<details>
<summary>Definitions</summary>
  
- The **broker** is an address that is specified in the backend settings and signs the trades and determines a 'broker fee' that the trader pays on their trade. It's the wallet of the 'whitelabelling partner'.
- **Referral codes** will give the trader a rebate on their broker fees that they get paid out as scheduled in the backend (e.g., once a week). The creator of the code also gets a cut of the fees that the trader paid.
- An **agency** is an address that refer to other addresses that then become agencies. If such a downstream agency creates a code, all agencies in the chain earn in relative terms from the trading fees paid by the trader that is using the referral code. The broker is an agency. 
  - Consequently, the broker is the root agency for all agencies. 
  - An agency can refer to many other addresses and make them an agency. 
  - No loops: An agency can only be referred to by one agency.
- A **referrer** is an address that created a code. A code can be created by anyone. If the code was created by an agency, the rebate depends on the entire chain of agencies and the code's specific pass-on percentage. If the code was created by an address that is not an agency, the trader rebate and referrer cut depends on the referrer's token holdings as specified in the broker backend settings.
  
</details>
  
## Get request: get all directly referred 'partners'
Who are my partners/codes that I assigned as an agency/referrer
and how much from my percentage cut do I pass on?

Returns 'downstream' partner addresses and directly entered codes and
the percentages that are passed on to them.
That is, the percentage obtained from `refer-cut` is 
further divided according to the `my-referrral` percentages.


http://127.0.0.1:8000/my-referrals?addr=0x0ab6527027ecff1144dec3d78154fce309ac838c

Example of an agency that is also a referrer. The code(s) 
that address 0x0a... created are
reported by their names, and the referred agencies are reported
by their addresses.

```
{
  "type": "my-referrals",
  "data": [
    {
      "referral": "0x20ec1a4332140f26d7b910554e3baaa429ca3756",
      "PassOnPerc": 10
    },
    {
      "referral": "AGENTUR",
      "PassOnPerc": 25
    },
    {
      "referral": "0xfacada864083eed4279dA1a9A7B1321E6102fD39",
      "PassOnPerc": 20
    }
  ]
}
```

No codes or agency referrals in the chain:
```
{
  "type": "my-referrals",
  "data": []
}
```

## Get request: percent fee rebate when trading with a code
What is the rebate I get as a trader per fees paid?

http://127.0.0.1:8000/code-rebate?code=DOUBLE_AG

Response:
`{"type":"code-rebate", "data":{"rebate_percent": 0.01}}`

The rebate is in percent, that is, 0.01 corresponds to 0.01% of the broker-fees
that will be rebated to traders that use this code

## Get request: percent fee passed-on to agency or referrer
How much fees can I distribute as an agency or referrer?

http://127.0.0.1:8000/refer-cut?addr=0x0ab6527027ecff1144dec3d78154fce309ac838c

A referrer without agency will have a fee rebate that is determined by their token holdings.
Therefore they should call the API with the amount of tokens they hold:
`http://127.0.0.1:8000/refer-cut?addr=0x0ab6527027ecff1144dec3d78154fce309ac838c&holdings=100000000000000000000000000`


```
{"type":"refer-cut", "data":{"isAgency":false, "passed_on_percent": 2.5}}
```
```
{"type":"refer-cut", "data":{"isAgency":true, "passed_on_percent": 4.000000000000001}}
```

The rebate is in percent, that is, 25.1 corresponds to 25.1% of the broker-fees
that were earned with all downstream referrals (downstream from the given address)  
are passed to the given agency/referral. Example: if the agency has 2 codes
further down the chain with 3 traders and they pay 450 MATIC in fees over a period,
this agency will receive 450 MATIC*25.1% and can pass on a share of it downstream.
What the agency actually earns then depends on how much is passed on downstream and
how much volume will be generated by the different codes downstream. 

## Get request: historical earnings

`http://127.0.0.1:8000/earnings?addr=0x5A09217F6D36E73eE5495b430e889f8c57876Ef3`

Available for any participant. We distinguish between earnings as a kick-back
for traders (asTrader=true), and earnings due to being a referrer/agency. 
```
{
  "type": "earnings",
  "data": [
    {
      "poolId": 1,
      "code": "DEFAULT",
      "earnings": 84.5672473711144,
      "asTrader": false,
      "tokenName": "MATIC",
      "since": "2024-07-11 16:24:55"
    },
    {
      "poolId": 2,
      "code": "DEFAULT",
      "earnings": 94.731112,
      "asTrader": false,
      "tokenName": "USDC",
      "since": "2024-07-11 16:24:55"
    },
    {
      "poolId": 2,
      "code": "DEFAULT",
      "earnings": 0.603999,
      "asTrader": true,
      "tokenName": "USDC",
      "since": "2024-07-11 16:24:55"
    }
  ]
}
```
## Get request: open payments for traders

Traders that are using a referral code get broker-fee rebates when trading via broker.
This endpoint shows how much fees the trader will be paid. 

http://127.0.0.1:8000/open-pay?traderAddr=0x85ded23c7bc09ae051bf83eb1cd91a90fae37366

```
{
  "type": "open-pay",
  "data": {
    "code": "THUANBX",
    "openEarnings": [
      {
        "poolId": 1,
        "earnings": 1.85133222663926,
        "tokenName": "MATIC"
      },
      {
        "poolId": 2,
        "earnings": 1.04185970407959,
        "tokenName": "USDC"
      }
    ]
  }
}
```
Error:
```
{"error":"Incorrect 'addr' parameter"}
```


## Get request: next payment date

`http://127.0.0.1:8000/next-pay`

```
{
  "type": "next-pay",
  "data": {
    "nextPaymentDueTs":1697544000,
    "nextPaymentDue":"2023-October-17 14:00:00"
    }
}
```

## Get request: code selection of a trader

http://127.0.0.1:8000/my-code-selection?traderAddr=0x85ded23c7bc09ae051bf83eb1cd91a90fae37366

a code selected:
`{"type":"my-code-selection","data":"THUANBX"}`

no code:
`{"type":"my-code-selection","data":""}`

## Get request: broker and executor address
http://127.0.0.1:8000/executor
```
{
  "executorAddress":"0x3ef256282e578c5D97a7231C3C046F19b1E50855",
  "brokerAddress":"0xb4111Fe4659057B01B28c3ff9Eb1349Fbf105e67"
}
```

## Post: Select a referral code as a trader

http://127.0.0.1:8000/select-code

```
 {
      "code": "THUANBX",
      "traderAddr": "0x0aB6527027EcFF1144dEc3d78154fce309ac838c",
      "createdOn": 1696264745,
      "signature": "0x4712cd2d1b772442afe1ea7f4c2a1f0cb8cc1bbc320a9bb449edd29f71e20ddd25b81daeb5a026f0d5e8361fe3739a365590ca9a6f416807ece8dd0ee3fd0a0e1b"
}
```
Success:
```
{
    "type": "select-code",
    "data": {
        "code": DOUBLE_AG
    }
}
```
Error:
`{"error":"code selection failed:Code already selected"}`, `{"error":"code selection failed:Failed"}`


<details>

<summary>Node SDK (>=0.9.7) </summary>

Select a referral code as a trader
```
let rc: APIReferralCodeSelectionPayload;
rc = {
    code: "ABCD",
    traderAddr: wallet.address,
    createdOn: 1696166434,
    signature: "",
  };
codeSigner = new ReferralCodeSigner(pk, wallet.address, RPC);
rc.signature = = await codeSigner.getSignatureForCodeSelection(rc);
```
</details>

## Get: rebates for referrers that are not an agency

http://127.0.0.1:8000/token-info

```
{"type":"token-info",
 "data":
 {"tokenAddr":"0xe05b86c761c70beab72fbfe919e5260e956cab99",
  "rebates":[
    {"cutPerc":0.2,"holding":0},
    {"cutPerc":1.5,"holding":100},
    {"cutPerc":2.5,"holding":1000},
    {"cutPerc":3.75,"holding":10000}
  ]
 }
}
```
## Get: settings
http://127.0.0.1:8000/settings


## Post: Update or create a code (anyone can be referrer)

http://127.0.0.1:8000/upsert-code

```
{
      "code": "ABCD",
      "referrerAddr": "0x0aB6527027EcFF1144dEc3d78154fce309ac838c",
      "createdOn": 1696166434,
      "PassOnPercTDF": 225,
      "signature": "0xb11b9af69b85719093be154bd9a9a23792d1ecb64f70b34dd69fdbec6c7cdf7048d62c6a6d94ee9f65e78aafad2ea45d94765e285a18485b879f814fde17c6b01b"
}
```
Success:
```
{
    "type": "upsert-code",
    "data": {
        "code": "ABCD"
    }
}
```
Error:
```
{"error":"code upsert failed:Not code owner"}
```

<details>

<summary>Node SDK (>=0.9.7) </summary>

```
let rp: APIReferralCodePayload;
let rcp: APIReferralCodePayload = {
      code: "ABCD",
      referrerAddr: wallet.address,
      createdOn: 1696166434,
      passOnPercTDF: 333,
      signature: "",
    };
codeSigner = new ReferralCodeSigner(pk, wallet.address, RPC);
rcp.signature = await codeSigner.getSignatureForNewCode(rcp);
```
</details>

## Post: Refer
/refer
passOnPercTDF is two-digit format, for example, 2.5% is sent as 250, 65% as 6500

```
{
    "parentAddr": "0x5A09217F6D36E73eE5495b430e889f8c57876Ef3",
    "referToAddr": "0x9d5aaB428e98678d0E645ea4AeBd25f744341a05",
    "passOnPercTDF": 225,
    "createdOn": 1696166434,
    "signature": "0x09bbe815eba739e28c665c6637e7e45dca03eed00d0eaffb1890713c8b3f9e760d41102d5d6885724bd53c7fc0bedcce8dfebe020464c234c1c1d4d194090f071c"
}
```
Success:
```
{
    "type": "referral-code",
    "data": {
        "referToAddr": "0x863ad9ce46acf07fd9390147b619893461036194"
    }
}
```
Error:
only one occurrence as child allowed:
`{"error":"referral failed:Refer to addr already in use"}`
`{"error":"referral failed:Not an agency"}`

<details>

<summary>Node SDK (>=0.9.7) </summary>

```
let rp: APIReferPayload = {
      parentAddr: wallet.address,
      referToAddr: "0x863ad9ce46acf07fd9390147b619893461036194",
      passOnPercTDF: 225,
      createdOn: 1696166434,
      signature: "",
    };
codeSigner = new ReferralCodeSigner(pk, wallet.address, RPC);
rp.signature= await codeSigner.getSignatureForNewReferral(rp);
```
</details>



## Dev: Contracts
Generate the ABI:
`abigen --abi src/contracts/abi/MultiPay.json --pkg contracts --type MultiPay --out multi_pay.go`
`abigen --abi src/contracts/abi/ERC20.json --pkg contracts --type Erc20 --out erc20.go`

## Run locally
- copy .envExample into .env and edit
- run `go run cmd/keygen/main.go ranky.txt` and copy ranky.txt into src/svc
- store the executor key in a file "./config/keyfile.txt", preceeded by 0x
- go run cmd/main.go


# Payment execution
See [here](README_PAY.md)

