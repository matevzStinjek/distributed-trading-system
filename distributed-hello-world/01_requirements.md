**Requirements Document: Mock Stock Trading & Portfolio Tracking System**

**Version:** 1.0
**Date:** 2025-04-01

**1. Introduction**

- **1.1. Purpose:** This document outlines the functional and non-functional requirements for a Mock Stock Trading & Portfolio Tracking System. The primary goal of this system is to serve as a practical learning platform for designing, building, and deploying distributed, horizontally scaled systems. It simulates core aspects of a trading system with simplified logic.
- **1.2. Scope:**
  - **In Scope:**
    - Processing of a simulated, real-time market data feed for mock stocks.
    - Display of (near) real-time mock stock prices to users.
    - User portfolio management (tracking mock cash and mock stock holdings).
    - Placement of mock "buy" and "sell" market orders.
    - Validation of orders against portfolio state (cash/holdings).
    - Simulation of order execution based on the market feed.
    - Recording and viewing of executed trade history.
    - Basic aggregated analytics on trading activity (e.g., volume).
  - **Out of Scope:**
    - Real monetary transactions or integration with real financial markets.
    - User registration and authentication mechanisms (assume users are pre-registered and authenticated).
    - Complex order types (e.g., limit orders, stop-loss).
    - Market maker simulation or complex order matching algorithms.
    - Dividend handling, stock splits, corporate actions.
    - Regulatory compliance features.
    - Sophisticated financial analytics or charting.
- **1.3. Definitions:**
  - **User:** An individual interacting with the system to trade mock stocks.
  - **Mock Stock/Symbol:** A fictional representation of a tradable asset identified by a unique symbol (e.g., "MOCKAAPL", "MOCKGOOG").
  - **Market Data Feed:** A continuous stream of simulated price updates for mock stocks.
  - **Portfolio:** A record associated with a user detailing their current mock cash balance and the quantity of each mock stock they own.
  - **Order:** An instruction from a user to buy or sell a specified quantity of a mock stock. Assumed to be a "market order" (executes at the prevailing price).
  - **Trade Execution:** The simulated fulfillment of an order, resulting in changes to the user's portfolio and a recorded trade event.
  - **Trade History:** A chronological record of a user's executed trades.
- **1.4. Target Audience:** System architects and software developers involved in the design and implementation of this learning platform.

**2. System Overview**

The Mock Stock Trading System allows registered users to simulate trading stocks using mock currency. Users can view a stream of changing stock prices, check their portfolio's value, place buy and sell orders, and review their past trading activity. The system processes orders based on available funds/shares and simulated market prices, updating portfolios accordingly. It also provides basic insights into overall market activity through aggregated analytics. The focus is on demonstrating distributed system patterns rather than financial accuracy.

**3. Functional Requirements**

- **FR1: Market Data Simulation & Display**

  - FR1.1: The system must ingest and process a continuous external stream of simulated stock price updates, each containing at least a stock symbol and a price.
  - FR1.2: The system must provide a mechanism for users (or client applications) to observe (near) real-time price changes for specific mock stocks they are interested in.
  - FR1.3: The system must maintain and make accessible the latest known price for each mock stock symbol present in the feed.

- **FR2: User Portfolio Management**

  - FR2.1: For each user, the system must securely store and manage their portfolio, consisting of a mock cash balance and the quantity held for each mock stock symbol.
  - FR2.2: Users must be able to query the system to view their current portfolio status (cash and all stock holdings).
  - FR2.3: Portfolio balances (cash and stock quantities) must be updated accurately and reliably upon the successful execution of trades.

- **FR3: Trade Order Placement**

  - FR3.1: Authenticated users must be able to submit buy orders specifying the stock symbol and quantity.
  - FR3.2: Authenticated users must be able to submit sell orders specifying the stock symbol and quantity.
  - FR3.3: **Buy Order Validation:** Before accepting a buy order, the system _must_ verify that the user's current cash balance is sufficient to cover the estimated cost (quantity \* latest known price).
  - FR3.4: **Sell Order Validation:** Before accepting a sell order, the system _must_ verify that the user currently holds at least the specified quantity of the target stock symbol in their portfolio.
  - FR3.5: Orders failing validation (FR3.3 or FR3.4) must be rejected immediately, and the user should be informed of the reason.
  - FR3.6: Valid orders must be accepted by the system and queued for execution processing. The user should receive confirmation that the order was accepted.

- **FR4: Trade Order Execution Simulation**

  - FR4.1: Accepted trade orders must be processed for execution in a timely manner. The order of execution for concurrent orders for the same user/stock does not need strict guarantees (FIFO is desirable but not mandatory).
  - FR4.2: Order execution price should be based on a reasonably current market price available at the time of execution processing. Price slippage simulation is not required.
  - FR4.3: **Atomic Portfolio Update:** Upon successful execution, the system _must atomically_ update the user's portfolio:
    - For a buy: Decrease cash balance, increase stock quantity.
    - For a sell: Increase cash balance, decrease stock quantity.
  - FR4.4: The system must create a persistent record (trade record) for each successfully executed trade, containing details like user ID, symbol, quantity, side (buy/sell), execution price, and execution timestamp.

- **FR5: Trade History & Analytics**
  - FR5.1: Users must be able to retrieve a chronological list of their own past executed trades.
  - FR5.2: The trade history view should allow basic filtering, at least by stock symbol.
  - FR5.3: The system must calculate and expose basic aggregated analytics, such as the total volume (number of shares) traded per stock symbol across all users over a defined period (e.g., last hour, current day).

**4. Non-Functional Requirements**

- **NFR1: Data Consistency**

  - **NFR1.1: Strong Consistency Required:**
    - The validation of an order (checking funds/shares per FR3.3, FR3.4) and the subsequent acceptance/reservation action _must appear atomic_ relative to other operations attempting to modify the same portfolio balance.
    - The update to a user's portfolio (cash and stock quantity change) resulting from a trade execution (FR4.3) _must be fully atomic_. Partial updates are unacceptable.
  - **NFR1.2: Eventual Consistency Acceptable:**
    - Real-time price display (FR1.2): Can tolerate minor delays (sub-second level acceptable). Users might see slightly stale prices.
    - Trade History view (FR5.1): Can lag slightly behind trade execution. A delay of a few seconds between execution and visibility in history is acceptable.
    - Aggregated Analytics (FR5.3): Can be calculated periodically or with delays. Does not need to reflect the absolute latest trade instantly.
    - Portfolio View (FR2.2): While updates must be atomic (NFR1.1), read requests for displaying the portfolio can tolerate minor delays (eventual consistency), although low latency is highly desirable.

- **NFR2: Unacceptable State Violations:** The system must prevent the following under all circumstances:

  - A user's cash balance becoming negative.
  - A user's holdings quantity for any stock becoming negative.
  - A sell order executing for more shares than the user possessed _at the time of execution_.
  - A buy order executing that results in a negative cash balance based on the _actual execution price_.
  - An accepted order being permanently lost without being executed or explicitly marked as failed/cancelled.
  - Inconsistent portfolio state (e.g., cash deducted for a buy, but shares never credited).

- **NFR3: Availability**

  - High availability is required for order placement (FR3) and portfolio viewing (FR2.2). Users should be able to trade and see their balance most of the time.
  - Moderate availability is acceptable for real-time price display (FR1.2) and trade history/analytics viewing (FR5). Brief interruptions or slightly increased delays are tolerable.
  - The market data ingestion (FR1.1) should be resilient, but the simulation source itself is outside the core system's availability requirement.

- **NFR4: Performance (Target Guidelines)**

  - Price updates broadcast to clients (FR1.2): Median latency < 500ms from market feed event to client display.
  - Order validation (FR3.3, FR3.4): P99 latency < 500ms.
  - Portfolio view (FR2.2): P99 latency < 500ms.
  - Trade execution processing (FR4.1-FR4.3): Latency can be higher; P99 < 5 seconds from order acceptance to portfolio update completion is acceptable for this simulation.
  - Trade history query (FR5.1): P99 latency < 1 second for typical user history.
  - Analytics query (FR5.3): P99 latency < 2 seconds.

- **NFR5: Scalability:** The system should be designed with horizontal scalability in mind to potentially handle:
  - Tens of thousands of concurrent users.
  - Thousands of distinct mock stock symbols.
  - High volume of price updates from the market feed.
  - High volume of orders and trades per second during peak times.

**5. Data Flow Overview (Conceptual)**

- **Market Data Flow:**
  `External Feed -> [Market Data Ingestion Service] -> [Event Bus (Price Updates)] -> [Real-time Push Service] -> User Interface`
  `[Event Bus (Price Updates)] -> [Price Caching Mechanism]`

- **Order Placement Flow:**
  `User Interface -> [API Gateway Service] -> [Order Management Service] -(Validate against Portfolio/Cache)-> [Event Bus (Valid Orders)] -> User Interface (Confirmation)`
  `(Validation uses data from Portfolio Store/Cache)`

- **Order Execution Flow:**
  `[Event Bus (Valid Orders)] -> [Order Execution Service] -(Gets current price from Cache)-> [Portfolio Update Logic] -> [Portfolio Data Store]`
  `[Portfolio Update Logic] -> [Event Bus (Executed Trades)]`

- **Data Persistence Flow:**
  `[Event Bus (Executed Trades)] -> [History Persistence Service] -> [Trade History Store]`
  `[Event Bus (Executed Trades)] -> [Analytics Processing Service] -> [Analytics Data Store]`

- **Data Retrieval Flow:**
  - **Portfolio:** `User Interface -> [API Gateway Service] -> [Portfolio Service] -> [Portfolio Data Store / Cache] -> User Interface`
  - **History:** `User Interface -> [API Gateway Service] -> [History Service] -> [Trade History Store] -> User Interface`
  - **Analytics:** `User Interface -> [API Gateway Service] -> [Analytics Service] -> [Analytics Data Store] -> User Interface`

**6. API Contract Requirements (Logical Interfaces)**

The system will require logical interfaces enabling communication between its components and potentially external clients. These define _intent_ rather than specific protocols/formats at this stage.

- **MarketData Interface:**
  - `SubscribePriceUpdates(symbols)`: Request a stream of price updates for specified symbols.
  - `GetLatestPrice(symbol)`: Retrieve the most recent known price for a symbol.
- **Order Interface:**
  - `PlaceOrder(user_id, symbol, quantity, side)`: Submit a new trade order. Returns status (accepted/rejected + reason) and potentially an order ID.
- **Portfolio Interface:**
  - `GetPortfolio(user_id)`: Retrieve the user's current cash and stock holdings.
  - _(Internal)_ `UpdatePortfolio(user_id, cash_delta, stock_symbol, quantity_delta)`: Atomically adjusts portfolio balances (likely used internally by the execution service).
- **TradeHistory Interface:**
  - `GetTrades(user_id, filter_criteria)`: Retrieve a list of executed trades based on user and filters (e.g., symbol, date range).
- **Analytics Interface:**
  - `GetVolumeAnalytics(symbol, time_window)`: Retrieve aggregated trading volume data.
