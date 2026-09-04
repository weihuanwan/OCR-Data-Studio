<template>
  <aside class="sidebar-left">
    <button @click="openFile" class="new-btn">
      <span class="plus">+</span>
      <span>新建解析</span>
    </button>

    <div class="tabs">
      <div
          class="tab"
          :class="{ active: activeTab === 'recent' }"
          @click="activeTab = 'recent'"
      >
        最近使用
      </div>

      <div
          class="tab"
          :class="{ active: activeTab === 'fav' }"
          @click="activeTab = 'fav'"
      >
        收藏夹
      </div>
    </div>

    <div class="search-row">
      <input
          v-model.trim="searchQuery"
          class="search-input"
          type="text"
          placeholder="搜索文件名 / 路径"
      />
      <button v-if="searchQuery" class="clear-btn" @click="searchQuery = ''">✕</button>
    </div>

    <div class="file-list">
      <div
          v-for="file in displayFiles"
          :key="file.id || file.path"
          class="file-item"
          :class="{ active: isActive(file) }"
          @click="$emit('select', file)"
      >
        <span class="dot" :class="{ active: isActive(file) }"></span>

        <div class="file-icon" :class="file.type">
          <span v-if="file.type === 'image'">🖼</span>
          <span v-else-if="file.type === 'pdf'">📕</span>
          <span v-else>📄</span>
        </div>

        <div class="file-info">
          <div class="file-name" :title="file.path">{{ file.name }}</div>
          <div class="file-date">{{ file.date }} · {{ file.size }}</div>
        </div>

        <button
            class="icon-btn fav-btn"
            :class="{ active: file.favorite }"
            :title="file.favorite ? '取消收藏' : '收藏'"
            @click.stop="$emit('toggle-favorite', file)"
        >
          ★
        </button>

        <button
            class="icon-btn del-btn"
            title="移除记录"
            @click.stop="$emit('remove-file', file)"
        >
          🗑
        </button>
      </div>

      <div v-if="displayFiles.length === 0" class="empty-tip">
        <template v-if="activeTab === 'fav'">
          暂无收藏文件
        </template>
        <template v-else-if="searchQuery">
          未搜索到相关文件
        </template>
        <template v-else>
          点击「新建解析」导入文件
        </template>
      </div>

      <div v-else class="no-more">没有更多数据</div>
    </div>

    <div class="sidebar-footer">
      <button @click="$emit('open-settings')" class="settings-btn" title="系统设置">
        <span>⚙️</span>
        <span>系统设置</span>
      </button>
    </div>
  </aside>
</template>

<script>
import { OpenFiles } from "../../wailsjs/go/main/App"

export default {
  name: 'SidebarLeft',
  props: {
    files: {
      type: Array,
      default: () => []
    },
    activeFile: {
      type: Object,
      default: null
    }
  },

  emits: ['select', 'add-files', 'toggle-favorite', 'remove-file', 'open-settings'],

  data() {
    return {
      activeTab: 'recent',
      searchQuery: ''
    }
  },

  computed: {
    displayFiles() {
      let list = Array.isArray(this.files) ? [...this.files] : []

      if (this.activeTab === 'fav') {
        list = list.filter(item => item.favorite)
      }

      const q = this.searchQuery.toLowerCase()
      if (q) {
        list = list.filter(item => {
          const name = String(item.name || '').toLowerCase()
          const path = String(item.path || '').toLowerCase()
          return name.includes(q) || path.includes(q)
        })
      }

      return list
    }
  },

  methods: {
    isActive(file) {
      if (!this.activeFile || !file) return false
      return this.activeFile.id === file.id || this.activeFile.path === file.path
    },

    openFile() {
      OpenFiles()
          .then(results => {
            if (!results || results.length === 0) return
            this.$emit('add-files', results)
          })
          .catch(err => {
            console.error('导入失败:', err)
          })
    }
  }
}
</script>

<style scoped>
.sidebar-left {
  width: 280px;
  background: #fff;
  border-right: 1px solid #e8e8e8;
  display: flex;
  flex-direction: column;
  flex-shrink: 0;
  height: 100%;
}

.new-btn {
  margin: 12px;
  padding: 9px;
  border: 1px solid #4f6ef7;
  background: #fff;
  color: #4f6ef7;
  border-radius: 6px;
  font-size: 14px;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  font-weight: 500;
  transition: all 0.2s;
}

.new-btn:hover {
  background: #f0f3ff;
}

.plus {
  font-size: 16px;
  font-weight: 300;
}

.tabs {
  display: flex;
  padding: 0 16px;
  border-bottom: 1px solid #f0f0f0;
  margin-bottom: 8px;
}

.tab {
  padding: 8px 0;
  margin-right: 16px;
  font-size: 13px;
  color: #666;
  cursor: pointer;
  position: relative;
}

.tab.active {
  color: #4f6ef7;
  font-weight: 500;
}

.tab.active::after {
  content: '';
  position: absolute;
  bottom: -1px;
  left: 0;
  right: 0;
  height: 2px;
  background: #4f6ef7;
  border-radius: 2px;
}

.search-row {
  padding: 0 12px 10px;
  display: flex;
  align-items: center;
  gap: 8px;
}

.search-input {
  flex: 1;
  height: 32px;
  border: 1px solid #d9d9d9;
  border-radius: 8px;
  padding: 0 10px;
  font-size: 13px;
  outline: none;
  box-sizing: border-box;
}

.search-input:focus {
  border-color: #4f6ef7;
}

.clear-btn {
  width: 28px;
  height: 32px;
  border: 1px solid #d9d9d9;
  background: #fff;
  border-radius: 8px;
  cursor: pointer;
  color: #666;
}

.clear-btn:hover {
  color: #333;
  background: #f5f5f5;
}

.file-list {
  flex: 1;
  overflow-y: auto;
  padding: 0 8px;
}

.file-item {
  display: flex;
  align-items: center;
  padding: 9px 8px;
  border-radius: 8px;
  cursor: pointer;
  margin-bottom: 4px;
  gap: 8px;
  border: 1px solid transparent;
}

.file-item:hover {
  background: #f5f7ff;
}

.file-item.active {
  background: #eef2ff;
  border-color: #dfe5ff;
}

.dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #4f6ef7;
  flex-shrink: 0;
  opacity: 0.6;
}

.dot.active {
  background: #52c41a;
  opacity: 1;
}

.file-icon {
  width: 34px;
  height: 34px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 6px;
  font-size: 16px;
  flex-shrink: 0;
}

.file-icon.image {
  background: #e6f7ff;
}

.file-icon.pdf {
  background: #fff1f0;
}

.file-icon.file {
  background: #f5f5f5;
}

.file-info {
  flex: 1;
  min-width: 0;
}

.file-name {
  font-size: 13px;
  color: #333;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.file-date {
  font-size: 11px;
  color: #999;
  margin-top: 2px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.icon-btn {
  width: 26px;
  height: 26px;
  border: none;
  background: transparent;
  border-radius: 6px;
  cursor: pointer;
  color: #b8bfd0;
  font-size: 14px;
  flex-shrink: 0;
  display: none;
}

.file-item:hover .icon-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

.fav-btn.active {
  color: #f7ba2a;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

.fav-btn:hover {
  color: #f7ba2a;
  background: #fff7e8;
}

.del-btn:hover {
  color: #ef4444;
  background: #fff1f1;
}

.empty-tip {
  text-align: center;
  padding: 40px 16px;
  font-size: 12px;
  color: #bbb;
}

.no-more {
  text-align: center;
  padding: 18px;
  font-size: 12px;
  color: #bbb;
}

.sidebar-footer {
  margin-top: auto;
  padding: 12px;
  border-top: 1px solid #e8e8e8;
  flex-shrink: 0;
}

.settings-btn {
  width: 100%;
  padding: 9px;
  border: 1px solid #d9dde5;
  background: #fff;
  color: #5d6676;
  border-radius: 6px;
  font-size: 13px;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  font-weight: 500;
  transition: all 0.2s;
}

.settings-btn:hover {
  background: #f7f8ff;
  color: #4f6ef7;
  border-color: #bfc9ff;
}
</style>