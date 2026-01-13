# Diary

## Step 1: Profiled Slow Queries

Used `EXPLAIN ANALYZE` to identify the worst performing queries in production.

### Top 5 offenders

| Query | Avg Time | Calls/day |
|-------|----------|-----------|
| `getUserOrders` | 2.3s | 50,000 |
| `searchProducts` | 1.8s | 120,000 |
| `getRecommendations` | 3.1s | 30,000 |
| `dashboardStats` | 4.5s | 5,000 |
| `reportGeneration` | 12s | 500 |

### Root causes identified

- Missing indexes on frequently filtered columns
- N+1 queries in ORM layer
- Full table scans on large tables
- No query result caching

## Step 2: Added Strategic Indexes

Created composite indexes based on query patterns observed in production logs.

### Indexes added

```sql
-- Orders lookup by user + status
CREATE INDEX idx_orders_user_status 
ON orders(user_id, status, created_at DESC);

-- Product search with category filter
CREATE INDEX idx_products_search 
ON products USING gin(to_tsvector('english', name || ' ' || description));

-- Recommendations by category + score
CREATE INDEX idx_recommendations_cat_score 
ON recommendations(category_id, score DESC);
```

### Results after indexing

- `getUserOrders`: 2.3s → **45ms** (98% improvement)
- `searchProducts`: 1.8s → **120ms** (93% improvement)
- `getRecommendations`: 3.1s → **80ms** (97% improvement)

## Step 3: Implemented Query Caching

Added Redis caching layer for expensive but stable queries.

### Cache strategy

```go
func GetDashboardStats(ctx context.Context, orgID string) (*Stats, error) {
    cacheKey := fmt.Sprintf("dashboard:%s", orgID)
    
    // Try cache first
    if cached, ok := redis.Get(ctx, cacheKey); ok {
        return cached.(*Stats), nil
    }
    
    // Compute fresh
    stats := computeStats(ctx, orgID)
    
    // Cache for 5 minutes
    redis.Set(ctx, cacheKey, stats, 5*time.Minute)
    
    return stats, nil
}
```

### Cache hit rates after 24h

- Dashboard stats: **94%** hit rate
- User preferences: **89%** hit rate
- Product catalog: **78%** hit rate

## Step 4: Connection Pooling Optimization

Tuned database connection pool settings to handle peak traffic better.

