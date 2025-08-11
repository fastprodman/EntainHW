# Entain Test task

The main goal of this test task is a develop the application for processing the incoming requests from the 3d-party providers.

The application must have an HTTP URL to receive incoming `POST` requests.
To receive the incoming POST requests the application must have an HTTP URL endpoint.
Technologies: Golang + Postgres.

## Requirements

Processing and saving incoming requests with balance recalculation.

Imagine that we have a users with the account balance.
Your application should have 2 routes:

`POST /user/{userId}/transaction` - Updates user balance
`GET /user/{userId}/balance` - Gets current user balance

The decision regarding database architecture, table structure and service architecture is made by you.

The application should be prepared for running via docker containers.
Best option will be running application via docker compose up -d without additional configuration.
Please, be informed that application without description about how to run and test it won't be accepted and reviewed.
Note: test task might be tested by automated tools.

## User Id
User id should be positive integer (`uint64`).
Please create predefined users, better with `userId` `1`, `2` and `3`.
So that calling `POST /user/1/transaction` or `GET /user/1/balance` on freshly started service will successfully do the job.

### Transaction route
Example of the POST request:
`POST /user/{userId}/transaction` HTTP/1.1

Headers:
`Source-Type: game`
`Content-Type: application/json`

Body payload:
```json
{"state": "win", "amount": "10.15", "transactionId": "some generated identificator"}
```

```json
{"state": "lose", "amount": "1.15", "transactionId": "some generated identificator"} 
```

Header "Source-Type" could be in 3 types (game, server, payment). This type probably can be extended in the future.

Body fields have to be:
```
{
  "state": string,
  "amount": string,
  "transactionId": string 
}
```

Possible values for `state` field are: (`win`, `lose`)
`win` - request must increase the user balance.
`lose` - request must decrease user balance.

`amount` field:
 - It has to be `string`.
 - Up to 2 decimal places will be sent.

**Response details**
The only requirement for response is `200 OK` HTTP status code in case of success, whatever else in case of error. 
Response payload can be defined by you in any form.

**NOTE:**

 - Each request (with the same `transactionId`) must be processed only once !
 - You should know that account balance can't be in a negative value.
 - The application must be competitive ability (it should process reasonable amount of transactions, 20-30 Requests per Second).

### User Balance route
Example of the POST request:
`GET /user/{userId}/balance` HTTP/1.1

Response should have 2 required fields in `json` format (if you need other fields - feel free to add):
```json
{
  "userId": 1, // uint64
  "balance": "9.25" // string, rounded to 2 decimal places
}
``