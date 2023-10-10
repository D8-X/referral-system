# Payment Execution

Payment execution is scheduled according to a CRON-synthax, for example every 3rd weekday at 14:00 UTC: `"paymentScheduleCron": "0 14 * * 2"`.

When executing payments, a batch timestamp is determined. This timestamp is stored in the database, together with the flag
whether the payment has finished or not. If the system crashes and it detects an unfinished payment in the database, 
the payment restarts for this batch (using the stored batch timestamp). When the payment completes processing all traders, completion is marked in the database.

So at the start of the program we check whether there is an unfinished payment and if so we start executing.