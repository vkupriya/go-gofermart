BEGIN TRANSACTION;

CREATE TABLE users(
    userid VARCHAR(200) UNIQUE NOT NULL,
    password VARCHAR(200) NOT NULL,
    accrual FLOAT NOT NULL,
    PRIMARY KEY (userid)
);

CREATE TABLE orders(
    userid VARCHAR(200) NOT NULL,
    number VARCHAR(200) UNIQUE NOT NULL,
    status VARCHAR(200) NOT NULL,
    accrual FLOAT NOT NULL,
    uploaded_at timestamp,
    PRIMARY KEY (number)
);

CREATE TABLE withdrawals(
    userid VARCHAR(200) NOT NULL,
    number VARCHAR(200) UNIQUE NOT NULL,
    sum FLOAT NOT NULL,
    processed_at timestamp
);

COMMIT;