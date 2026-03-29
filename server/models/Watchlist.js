const mongoose = require('mongoose');

const watchlistSchema = new mongoose.Schema({
  userId: {
    type: mongoose.Schema.Types.ObjectId,
    ref: 'User',
    required: true
  },
  fundCode: {
    type: String,
    required: true
  },
  fundName: {
    type: String,
    required: true
  },
  alertThreshold: {
    type: Number,
    default: 5  // 默认5%涨幅提醒
  },
  addedAt: {
    type: Date,
    default: Date.now
  }
});

// 复合唯一索引：一个用户不能重复标记同一个基金
watchlistSchema.index({ userId: 1, fundCode: 1 }, { unique: true });

module.exports = mongoose.model('Watchlist', watchlistSchema);
