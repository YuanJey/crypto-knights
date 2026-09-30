# Deribit 到期日持仓演化看板设计

状态：草案

版本：v1

更新日期：2026-09-30

## 1. 背景

当前 Options Service 只接收标准化逐笔成交，并按权利金聚合方向信号。现有数据无法展示 Deribit 期权在某个到期日下，Call、Put 以及各执行价持仓量随时间的变化。

本设计在整体交易看板中增加“到期日持仓演化”模块。用户先查看各到期日的当前状态，再点击某个到期日，在同一模块内查看从历史数据起点到该合约到期时刻的时间序列。

该模块描述公开市场中的持仓分布和市场状态，不推断具体账户的真实持仓或交易动机。

## 2. 目标

v1 需要回答以下问题：

1. 当前哪些到期日聚集了最多 OI？
2. 某个到期日的 Call、Put 和总 OI 如何随时间变化？
3. 哪些时间段发生了明显增仓或减仓？
4. 哪些执行价正在快速积累或释放持仓？
5. 标的价格变化与期权持仓变化是否同时发生？
6. 当前图表数据是否完整、及时，是否存在采集缺口？

## 3. 非目标

v1 不负责：

- 根据公开 OI 判断具体交易者是对冲还是方向押注。
- 将 Call OI 直接解释为看涨，或将 Put OI 直接解释为看跌。
- 还原系统开始采集之前不存在的历史 OI。
- 预测当前时间到到期日之间的未来 OI。
- 根据 Max Pain、Gamma Exposure 等单一指标生成交易指令。
- 在看板中直接执行期权交易。

## 4. 核心概念与口径

### 4.1 到期日

到期日使用 Deribit 合约元数据中的准确到期时间，不根据合约名称自行推算。内部统一使用 UTC 存储，界面可切换显示时区。

同一标的和到期时刻下的全部 Call、Put 合约组成一个到期日集合：

```text
(venue, underlying, expiry)
```

### 4.2 Open Interest

OI 使用 Deribit 公共市场数据报告的未平仓合约数量。每张未平仓合约同时存在多头和空头，因此 OI 只表示存量，不表示市场净多空。

默认单位为合约数量。用户可以切换：

- `contracts`：合约数量
- `underlying`：折算后的标的币数量
- `usd`：按采样时点指数价格折算的 USD 名义价值

USD OI 会受到标的价格变化影响。即使合约数量没有变化，USD OI 也可能发生变化，因此界面默认不使用 USD。

### 4.3 OI 变化

时间桶的 OI 变化定义为：

```text
delta_oi = close_oi(current_bucket) - close_oi(previous_bucket)
```

百分比变化定义为：

```text
delta_oi_percent = delta_oi / close_oi(previous_bucket)
```

前一时间桶 OI 为零时，不计算百分比。

### 4.4 OI Candle

OI Candle 由时间桶内的 OI 状态构造：

```text
open  = 时间桶内第一条有效 OI
high  = 时间桶内最高 OI
low   = 时间桶内最低 OI
close = 时间桶内最后一条有效 OI
```

OI Candle 只是存量变化的可视化形式，不具有价格 K 线的全部市场含义。v1 默认展示 OI 折线，Candle 作为可选视图。

### 4.5 成交量与资金流边界

快照可以保存 Deribit 在采样时点报告的 `volume_24h`，但这是滚动 24 小时指标，不能当作当前时间桶的成交量。

主动买卖资金流必须来自逐笔成交，不能从周期快照准确还原。v1 快照看板不展示主动买卖资金流；后续如需叠加，继续使用独立的逐笔成交数据链路。

## 5. 信息架构

该功能是整体 Dashboard 中的一个模块，不拆分为独立应用。

模块包含三个层次：

1. 到期日概览。
2. 已选到期日的时间序列。
3. 单执行价详情。

点击到期日后，在模块内部更新图表，保留当前标的、指标、时间范围和执行价筛选。URL 查询参数应同步更新，保证页面可以刷新和分享。

建议状态参数：

```text
underlying=BTC
expiry=2026-10-30T08:00:00Z
metric=open_interest
range=30d
interval=1h
strike_scope=all
```

## 6. 页面布局

### 6.1 模块框架

```text
┌─────────────────────────────────────────────────────────────────────┐
│ 到期日持仓演化         BTC | ETH       UTC       快照状态           │
├─────────────────────────────────────────────────────────────────────┤
│ 到期日  [02 OCT] [09 OCT] [30 OCT] [27 NOV] [25 DEC]  [更多]       │
├─────────────────────────────────────────────────────────────────────┤
│ 30 OCT 2026 · DTE 30   总 OI   24h ΔOI   Put/Call   ATM IV         │
│                                                                     │
│ [持仓] [IV] [24h成交量]  [绝对值|变化率]  [24H][7D][30D][全周期]  │
│                                                                     │
│ ① 标的指数价格曲线                                                │
│ ─────────────────────────────────────────────────────────────────  │
│ ② Call / Put / Total OI 时间曲线                                 │
│ ─────────────────────────────────────────────────────────────────  │
│ ③ Call / Put ΔOI 柱状图                                           │
├─────────────────────────────────────────────────────────────────────┤
│ 执行价筛选   全部 | ATM ±10% | Top 10 OI | 自定义                 │
└─────────────────────────────────────────────────────────────────────┘
```

### 6.2 到期日选择器

每个到期日显示：

- 到期日期
- 剩余天数 DTE
- 当前总 OI
- 24 小时 OI 变化
- Put/Call OI 比例

排序规则：

1. 未到期合约按到期时间升序。
2. 已到期合约进入“历史”区域，按到期时间降序。
3. 当前选中项保持明显选中状态，但不使用涨跌色表达 Call/Put。

到期日较多时，首屏显示最近期限和 OI 最大的期限，其余通过菜单选择。选择到期日不得触发整页跳转。

### 6.3 摘要栏

选择到期日后展示：

- 总 OI
- Call OI
- Put OI
- 1 小时和 24 小时 `delta_oi`
- Put/Call OI Ratio
- 24 小时成交量
- ATM IV
- 数据最后更新时间

所有摘要指标必须使用同一个 `as_of` 时间，避免不同时间快照混合。

## 7. 核心时间序列图

### 7.1 联动图组

默认“持仓”视图由三个上下排列的子图组成：

| 子图 | 建议高度 | 内容 | Y 轴 |
| --- | ---: | --- | --- |
| 价格 | 25% | 标的指数价格 | USD 价格 |
| 持仓 | 55% | Call、Put、Total OI | 合约、标的币或 USD |
| 变化 | 20% | Call、Put `delta_oi` | 与 OI 相同单位 |

三个子图必须：

- 共用同一个 X 轴范围。
- 使用同一批完整快照的数据。
- 保持垂直网格线对齐。
- 共享缩放、平移和十字光标状态。
- 仅在底部子图显示完整时间刻度，减少重复信息。

标的价格不能与 OI 共用 Y 轴，也不默认使用双 Y 轴叠加。不同量纲独立成图，避免通过缩放比例制造并不存在的相关性。

### 7.2 X 轴

X 轴表示事件时间，最大边界不得晚于到期时刻。

时间范围：

| 范围 | 默认粒度 | 说明 |
| --- | --- | --- |
| `24H` | `5m` | 短期变化 |
| `7D` | `15m` | 周内变化 |
| `30D` | `1h` | 中期变化 |
| `全周期` | 自动 | 从最早可用数据到到期 |

活动合约在“全周期”模式下：

- 图表右边界固定为到期时刻。
- 当前时间使用垂直线标记。
- 当前时间之后使用灰色背景表示“尚未发生”。
- 未来区间不得插值、外推或生成零值。

普通 `24H`、`7D`、`30D` 模式的右边界为：

```text
min(now, expiry)
```

已到期合约的右边界固定为到期时刻。

时间范围变化必须同时作用于三个子图。任一子图都不能拥有独立的时间缩放状态。

### 7.3 标的价格子图

价格子图默认展示 `index_price` 折线。每个时间节点的价格必须来自与 OI 相同的完整快照批次：

```text
index_price(t) =
  median(non-null index_price)
  where batch.status = complete
    and snapshot_at = t
    and expiry = selected_expiry
```

使用中位数可以避免单条异常合约记录污染价格曲线。如果同一批次内的 `index_price` 差异超过配置容忍值，该时间点需要标记数据异常。

价格子图规则：

- 默认使用独立的自动缩放 Y 轴。
- Y 轴必须显示明确价格范围，不能隐藏刻度。
- 数据缺口处断线，不跨缺口连线。
- 仅凭 5 分钟快照生成采样价格曲线，不标记为真实 OHLC K 线。
- 后续接入独立行情 K 线时，可以在相同子图中切换折线和 K 线。

### 7.4 OI 主图

OI 主图默认展示：

- Call OI 折线
- Put OI 折线
- Total OI 折线

默认单位为合约数量，Y 轴从零开始。用户切换为“变化率”后，以当前可视区间第一个有效点为基准 `100`：

```text
indexed_oi(t) = oi(t) / first_visible_oi * 100
```

变化率模式用于比较 Call 和 Put 的相对变化速度，不能显示成收益率。

### 7.5 OI 变化子图

变化子图使用零轴上下柱状图：

- Call `delta_oi`
- Put `delta_oi`
- Total `delta_oi` 通过图例按需开启

Y 轴上下范围保持对称，避免相同绝对值的增仓和减仓呈现不同视觉强度。缺少前一个完整快照时，该时间点的 `delta_oi` 返回 `null`，不能填零。

### 7.6 指标模式

顶部指标控制器切换中间主图和底部辅助图，价格子图始终保留：

| 模式 | 中间主图 | 底部辅助图 |
| --- | --- | --- |
| `持仓` | Call、Put、Total OI | Call、Put `delta_oi` |
| `IV` | ATM、25 Delta Call、25 Delta Put IV | 25 Delta Risk Reversal |
| `24h成交量` | Call、Put、Total `volume_24h` | 不显示 |

`volume_24h` 是每次快照记录的滚动 24 小时值，必须明确显示“24h”，不能标记为当前时间桶成交量，也不能拆分为主动买入和主动卖出。

切换指标模式时必须保留选中的到期日、执行价范围、时间窗口和十字光标时间。

### 7.7 执行价筛选

支持以下范围：

- `all`：该到期日全部执行价聚合
- `atm_range`：当前 ATM 上下指定百分比
- `top_oi`：当前 OI 最大的前 N 个执行价
- `custom`：用户手动选择一个或多个执行价

`top_oi` 默认最多显示 10 条执行价曲线，避免图表不可读。Call 和 Put 必须使用固定颜色体系，同一执行价通过线型或标记区分，不依赖颜色数量无限扩展。

### 7.8 图表交互

- 鼠标滚轮或触控板缩放时间范围。
- 拖动平移，但不得越过数据起点和到期时间。
- 双击恢复当前时间范围。
- 在任意子图移动十字光标时，其他子图同步到相同 `snapshot_at`。
- 点击数据点后锁定时间节点，再次点击空白区域解除锁定。
- 图例支持隐藏单个序列。
- 图表切换指标时保持时间范围和选中到期日。

联动 Tooltip 固定显示在图表右上角，至少包含：

```text
时间                 2026-10-20 12:00 UTC
距离到期             10 天 20 小时
标的指数价格         68,420 USD
相邻快照价格变化     +1.2%
Call OI              18,240
Put OI               21,510
Total OI             39,750
Call / Put delta_oi  +320 / +960
24h 成交量           3,420
ATM IV               52.8%
数据状态             complete
```

所有值必须来自同一 `snapshot_at`。不能把最近价格与较早 OI 拼接到同一个 Tooltip。

### 7.9 数据缺口联动

当某个快照批次缺失或不完整时：

- 三个子图在相同时间区间同时断开。
- 缺口区域使用统一阴影。
- Tooltip 显示缺失批次的起止时间。
- 不使用价格数据单独填补 OI 缺口。
- 不使用 OI 数据单独填补价格缺口。

这项约束保证用户看到的价格和持仓始终来自同一个时间节点。

### 7.10 移动端

移动端保持三个子图上下排列，不改成重叠双轴图。到期日和时间范围使用横向滚动控件，Tooltip 改为点击锁定，图表最小高度不得因隐藏图例发生变化。

### 7.11 颜色语义

- Call：蓝色
- Put：橙色
- Total：中性色
- OI 增加和减少：使用独立的正负变化颜色
- 标的指数价格：使用中性色

不能用绿色 Call、红色 Put 暗示 Call 必然看涨、Put 必然看跌。

## 8. 数据采集

### 8.1 数据类型

每个快照需要覆盖当时全部有效期权合约，并保存：

- 合约名称、标的、到期时间、执行价和 Call/Put 类型
- 合约大小
- Open Interest
- Mark Price 和 Mark IV
- Delta、Gamma、Vega
- Bid/Ask 价格和数量
- 标的指数价格和合约对应的 Underlying Price
- Deribit 报告的 24 小时成交量
- 逻辑快照时间和实际采集时间

### 8.2 采集策略

v1 使用周期性全量快照，不维护逐条 OI 变化事件：

1. 每 5 分钟创建一个逻辑快照批次。
2. 读取指定标的的全部有效期权合约。
3. 将返回结果标准化为一份明细记录对应一个合约。
4. 在同一数据库事务中写入快照明细并更新批次计数。
5. 只有实际数量符合完整性要求时，批次状态才更新为 `complete`。
6. 图表查询默认只读取 `complete` 批次。

逻辑快照时间统一向下对齐到采集周期：

```text
snapshot_at = floor(collection_started_at, 5 minutes)
```

同一交易所、标的和 `snapshot_at` 只能存在一个批次。任务重试必须复用该批次，不得重复产生同一个时间点的数据。

5 分钟是 v1 默认值，可以通过后端配置调整。前端不得自行声明高于实际采集频率的时间精度。

### 8.3 批次完整性

每个批次记录：

- 预期合约数量
- 实际写入数量
- 开始和完成时间
- `collecting`、`complete`、`partial` 或 `failed` 状态
- 可选错误信息

完整性规则：

```text
status = complete
当且仅当：
  actual_count = expected_count
  且批次内不存在重复 instrument_name
  且所有必填字段通过校验
```

如果采集期间 Deribit 新增或停用合约，采集器应重新获取有效合约集合并重试当前批次。无法确认完整性时必须保留为 `partial`，不能当作 OI 下降。

### 8.4 数据缺口

采集失败期间不得使用最后一个值无限填充。相邻两个完整批次的时间差超过预期周期加容忍时间时，API 必须返回数据缺口。

每条时间序列返回数据质量状态：

- `complete`
- `partial`
- `stale`
- `unavailable`

图表使用断线或阴影展示缺口，不把缺失数据绘制为零。

### 8.5 到期处理

到期前最后一个完整批次需要长期保留。到期后不再继续采集该合约；图表右边界使用合约到期时刻，最后数据点使用到期前最近的完整快照。

界面根据合约到期时间展示结算标记。不能人为补写一条 OI 为零的快照，否则会把结算表现成普通减仓。

## 9. 聚合算法

### 9.1 到期日聚合

一个完整批次已经包含当时全部有效合约。因此，在快照时间 `t`，到期日总 OI 直接对该批次内相同到期时间的明细求和：

```text
expiry_oi(t) =
  sum(open_interest)
  where batch.status = complete
    and snapshot_at = t
    and expiry = selected_expiry
```

Call、Put 分别聚合：

```text
call_oi(t) = sum(call instrument oi at t)
put_oi(t)  = sum(put instrument oi at t)
total_oi(t) = call_oi(t) + put_oi(t)
```

不能跨多个快照直接对 `open_interest` 求和，否则会把同一持仓重复累计。

### 9.2 时间范围重采样

底层快照粒度默认为 5 分钟。API 根据查询范围重采样：

- `24H`：直接使用 5 分钟快照
- `7D`：聚合为 15 分钟
- `30D`：聚合为 1 小时
- `全周期`：自动选择 1 小时或 1 天

每个展示时间桶先计算每次快照的到期日 OI，再从这些快照总值计算 OHLC：

```text
open  = 桶内第一个完整快照的 expiry_oi
high  = 桶内所有完整快照 expiry_oi 的最大值
low   = 桶内所有完整快照 expiry_oi 的最小值
close = 桶内最后一个完整快照的 expiry_oi
```

API 返回不超过约 2,000 个数据点。前端不得下载全部快照后自行完成全周期聚合。

### 9.3 执行价聚合

执行价维度使用：

```text
(underlying, expiry, strike, option_type, snapshot_at)
```

同一执行价的 Call 和 Put 独立统计。不能用 Call OI 减 Put OI 生成“净持仓”，因为二者的风险含义和 Delta 不对称。

### 9.4 衍生指标

合约数量换算：

```text
underlying_oi = sum(open_interest * contract_size)
usd_oi = sum(open_interest * contract_size * index_price)
```

Put/Call OI Ratio：

```text
put_call_oi_ratio = put_oi / call_oi
```

当 `call_oi` 为零时返回 `null`，不能返回无穷大或零。

ATM IV 使用执行价最接近 `underlying_price` 的 Call 和 Put Mark IV 计算；如果其中一侧缺失，只返回可用侧并降低数据质量状态。25 Delta IV 使用实际 Delta 最接近 `+0.25` 的 Call 和最接近 `-0.25` 的 Put，不根据执行价比例近似。

## 10. 数据模型

正式开发遵循 DDL 先行。v1 为该看板只新增两张表：快照批次表和快照明细表。现有逐笔成交能力保持独立，不纳入本次快照表设计。

### 10.1 `option_snapshot_batches`

每个交易所、标的和逻辑快照时间对应一条批次记录：

```sql
CREATE TABLE option_snapshot_batches (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    venue           VARCHAR(32) NOT NULL,
    underlying      VARCHAR(16) NOT NULL,
    snapshot_at     TIMESTAMPTZ NOT NULL,
    started_at      TIMESTAMPTZ NOT NULL,
    completed_at    TIMESTAMPTZ,
    expected_count  INTEGER NOT NULL CHECK (expected_count >= 0),
    actual_count    INTEGER NOT NULL DEFAULT 0 CHECK (actual_count >= 0),
    status          VARCHAR(16) NOT NULL CHECK (
        status IN ('collecting', 'complete', 'partial', 'failed')
    ),
    error_message   TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (venue, underlying, snapshot_at)
);
```

职责：

- 保证同一时间槽只有一个快照。
- 判断整批数据是否可以参与聚合。
- 记录采集延迟和缺失状态。
- 支持失败重试和数据质量审计。

### 10.2 `option_chain_snapshots`

一条记录表示某个期权合约在某个完整快照中的市场状态：

```sql
CREATE TABLE option_chain_snapshots (
    batch_id          BIGINT NOT NULL REFERENCES option_snapshot_batches(id),
    snapshot_at       TIMESTAMPTZ NOT NULL,
    venue             VARCHAR(32) NOT NULL,
    underlying        VARCHAR(16) NOT NULL,
    instrument_name   VARCHAR(96) NOT NULL,
    expiry            TIMESTAMPTZ NOT NULL,
    strike            NUMERIC(38, 18) NOT NULL CHECK (strike > 0),
    option_type       VARCHAR(4) NOT NULL CHECK (option_type IN ('call', 'put')),
    contract_size     NUMERIC(38, 18) NOT NULL CHECK (contract_size > 0),
    open_interest     NUMERIC(38, 18) NOT NULL CHECK (open_interest >= 0),
    mark_price        NUMERIC(38, 18),
    mark_iv           NUMERIC(20, 10),
    index_price       NUMERIC(38, 18),
    underlying_price  NUMERIC(38, 18),
    delta             NUMERIC(20, 10),
    gamma             NUMERIC(30, 18),
    vega              NUMERIC(30, 18),
    bid_price         NUMERIC(38, 18),
    bid_amount        NUMERIC(38, 18),
    ask_price         NUMERIC(38, 18),
    ask_amount        NUMERIC(38, 18),
    volume_24h        NUMERIC(38, 18),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (snapshot_at, batch_id, instrument_name)
) PARTITION BY RANGE (snapshot_at);
```

推荐索引：

```sql
CREATE INDEX idx_option_snapshots_expiry_time
    ON option_chain_snapshots (underlying, expiry, snapshot_at DESC);

CREATE INDEX idx_option_snapshots_strike_time
    ON option_chain_snapshots (
        underlying,
        expiry,
        strike,
        option_type,
        snapshot_at DESC
    );

CREATE INDEX idx_option_snapshots_batch
    ON option_chain_snapshots (batch_id);
```

`snapshot_at`、`venue` 和 `underlying` 与批次表存在适度冗余，这是为了支持时间分区和看板查询，写入时必须校验它们与批次记录一致。

`mark_price`、Greeks 和盘口字段允许为空，因为深度不足的远端执行价可能没有完整报价。`open_interest`、到期日、执行价和类型属于核心字段，不允许为空。

### 10.3 分区与扩展

默认使用 PostgreSQL，并按 `snapshot_at` 进行月度范围分区。环境支持时可以将明细表改为 TimescaleDB hypertable，但 API 和领域模型不得依赖 TimescaleDB 专有语义。

v1 不创建 `expiry_metric_bars` 或 `strike_metric_bars`。先通过索引和查询时聚合验证实际数据量；只有在性能指标无法满足时，再增加物化视图或聚合表，且不改变对外 API。

## 11. API 设计

### 11.1 到期日列表

```http
GET /v1/expiries?underlying=BTC&include_expired=false
```

返回每个到期日的当前摘要和统一的 `as_of` 时间。

### 11.2 到期日时间序列

```http
GET /v1/expiries/{expiry}/timeline
    ?underlying=BTC
    &metric=open_interest
    &interval=1h
    &from=2026-09-01T00:00:00Z
    &to=2026-10-30T08:00:00Z
    &strike_scope=all
```

响应必须包含：

- `expires_at`
- `data_available_from`
- `data_available_to`
- `server_time`
- `interval`
- `snapshot_interval`
- `unit`
- `price_series`
- `metric_series`
- `gaps`

`price_series` 和 `metric_series` 必须按相同时间桶对齐。每个时间点都携带对应的 `snapshot_at`；缺少任一侧数据时，该时间点整体标记为空，不允许跨时间拼接。

v1 支持的 `metric`：

- `open_interest`
- `open_interest_change`
- `volume_24h`
- `mark_iv`

当请求粒度小于实际快照周期时，API 返回 `400 Bad Request`，不能通过插值伪造更高精度数据。

### 11.3 到期日执行价快照

```http
GET /v1/expiries/{expiry}/strikes
    ?underlying=BTC
    &at=2026-09-30T12:00:00Z
```

用于查看十字光标所在时间点的执行价分布。服务端选择小于或等于 `at` 的最近完整批次，并在响应中返回真实 `as_of` 时间。

### 11.4 快照状态

```http
GET /v1/snapshots/status?underlying=BTC
```

返回最新批次时间、状态、预期数量、实际数量和最近的数据缺口。Dashboard 使用该接口展示快照的新鲜度，不直接读取内部批次表。

所有金额和数量继续使用十进制字符串，保持与现有 Options Service API 一致。

## 12. 服务边界

保持 Options Service 对期权领域数据和查询 API 的所有权：

```text
Deribit Public Market Data
          |
          v
options-service/cmd/collector
          |
          v
PostgreSQL
  - option_snapshot_batches
  - option_chain_snapshots
          |
          v
options-service/cmd/server
  - 查询时聚合
  - 可选结果缓存
          |
          v
apps/dashboard
```

采集器与 HTTP Server 可以使用不同进程，但属于同一个 Options Service 模块。前端不得直接连接 Deribit，以免出现数据口径、重连和历史存储不一致。

当前内存 Store 只适合原型验证。该功能需要持久化数据库，否则服务重启后无法展示历史演化。

## 13. 性能与保留策略

设计目标：

- 到期日列表 API 的缓存响应 P95 小于 300ms。
- 单个时间序列 API 的缓存响应 P95 小于 500ms。
- 单次响应不超过约 2,000 个时间点。
- 最新完整快照正常情况下不晚于当前时间 6 分钟。

建议保留：

- 完整 5 分钟快照：至少保留至对应合约到期后 180 天。
- `partial` 和 `failed` 批次元数据：至少保留 90 天。
- 不完整批次的明细：问题排查完成后可以清理。
- 每个到期日最后一个完整快照：长期保留。

保留期限和采样频率必须配置化，不得写死在前端。上线后根据实际行数、索引大小和查询耗时决定是否增加压缩、物化视图或长期聚合表。

## 14. 空状态与异常状态

### 无历史数据

显示最早可用时间，不绘制伪造历史：

```text
该到期日的数据从 2026-09-30 08:00 UTC 开始采集
```

### 数据延迟

显示最后完整快照时间和 `stale` 状态，暂停自动刷新动画。

### 部分缺失

图表断线，并在缺口区间显示数据质量提示。

### 到期日已结算

停止自动刷新，保留最终到期前快照和结算标记。

### 无报价但有 OI

OI 曲线正常展示，价格、IV 或盘口字段显示为空。不能因报价字段为空而丢弃有效 OI。

## 15. 分阶段交付

### Phase 1：数据基础

- `option_snapshot_batches` DDL
- `option_chain_snapshots` DDL
- 每 5 分钟采集一次完整期权链
- 批次完整性校验和幂等重试
- PostgreSQL 分区、索引和保留策略
- 到期日与执行价查询时聚合

### Phase 2：看板 MVP

- BTC、ETH 到期日选择器
- 到期日摘要
- 标的指数价格曲线
- Call、Put、Total OI 时间曲线
- `delta_oi` 柱状图
- 24 小时滚动成交量和 IV
- 时间范围、粒度和单位切换
- 数据缺口及过期状态

### Phase 3：执行价分析

- Top OI 和自定义执行价
- 执行价时间序列
- 单合约详情抽屉

### Phase 4：高级分析

- IV Surface 和 Skew
- 增仓、减仓和移仓形态识别
- OI 变化动态异常分位数
- 告警和历史变化回测
- 另行设计逐笔成交与主动资金流叠加

## 16. 验收条件

1. 用户可以在 Dashboard 模块中切换标的和到期日。
2. 点击到期日后图表原位更新，不刷新整个页面。
3. 时间轴数据和视图区间均不得超过到期时刻。
4. 活动合约的未来区间不生成预测值或零值。
5. Call、Put、Total OI 与同一时刻的底层快照聚合一致。
6. `delta_oi` 使用相邻时间桶收盘值计算。
7. 用户可以切换 `24H`、`7D`、`30D` 和全周期。
8. 用户可以切换合约、标的币和 USD 单位，并看到单位说明。
9. 数据缺口、延迟和结算必须在图表中明确展示。
10. 标的价格、OI 和变化量图共享时间轴与十字光标。
11. 选择单个执行价后可以查看其 Call/Put 历史变化。
12. 已到期合约可以从历史区域重新打开。
13. 服务重启不会丢失已经采集的历史数据。
14. 看板不得将公开 OI 标记为确定的对冲或方向押注。
15. `partial` 和 `failed` 批次不得参与默认聚合。
16. 同一交易所、标的和快照时间重复采集不会产生重复数据。
17. 指定历史时间查看执行价时，API 返回不晚于该时间的最近完整快照。

## 17. 待确认事项

以下项目不阻塞设计，但应在开发前确认：

- v1 是否只支持 BTC、ETH，还是同时支持 SOL、XRP。
- PostgreSQL 是否启用 TimescaleDB 扩展。
- 5 分钟完整快照的最终保留期限。
- 首版是否需要 OI Candle，还是只提供折线。
- Dashboard 前端的具体技术栈和图表库。

默认建议：

- v1 先支持 BTC、ETH。
- 使用 PostgreSQL；可用时启用 TimescaleDB。
- 默认折线，OI Candle 后置。
- 图表使用 Apache ECharts，统一实现折线、柱状图和 OI Candle。
