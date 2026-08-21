package tracing

import (
	"context"
	"time"

	"github.com/QuantumNous/new-api/relaykit/dto"
)

// billingFactsKey 是 BillingFacts 在 context 中的键，避免与其他包冲突。
type billingFactsKey struct{}

// BillingFacts 承载一次模型操作在结算时（§9.7）才可用的最终事实：
// 归一化 Usage、实际消耗额度、以及（可选）流式首字延迟。
// 这些事实由 Handler 在 settlement 汇聚点写入，GenAI Span 结束后读取。
type BillingFacts struct {
	Usage  *dto.BillingUsage
	Quota  int64
	TTFC   time.Duration
	HasTTFC bool
}

// newBillingFacts 在 ctx 上挂载一个可变的 BillingFacts 指针，返回新的 ctx 与指针。
// spanCtx 持有该指针，Handler 通过 FromBillingFacts(ctx.Request.Context()) 定位它。
func newBillingFacts(ctx context.Context) (context.Context, *BillingFacts) {
	f := &BillingFacts{}
	return context.WithValue(ctx, billingFactsKey{}, f), f
}

// FromBillingFacts 从 ctx 中取出 BillingFacts；未挂载时返回 nil。
func FromBillingFacts(ctx context.Context) *BillingFacts {
	if f, ok := ctx.Value(billingFactsKey{}).(*BillingFacts); ok {
		return f
	}
	return nil
}

// RecordBillingFacts 由 Handler 在 settlement 汇聚点写入最终结算事实（§9.7）。
// OTel 未启用或 ctx 未挂载 BillingFacts 时为空操作，不改变结算语义。
func RecordBillingFacts(ctx context.Context, usage *dto.BillingUsage, quota int64) {
	if f := FromBillingFacts(ctx); f != nil {
		if usage != nil {
			f.Usage = usage
		}
		f.Quota = quota
	}
}

// RecordTimeToFirstChunk 记录流式首字延迟（相对请求开始）。仅记录可靠值。
func RecordTimeToFirstChunk(ctx context.Context, ttfc time.Duration, has bool) {
	if f := FromBillingFacts(ctx); f != nil {
		f.TTFC = ttfc
		f.HasTTFC = has
	}
}
