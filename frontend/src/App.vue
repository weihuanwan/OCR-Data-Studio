<template>
  <div class="app-root">
    <div class="workspace">
      <SidebarLeft
          :files="fileList"
          :activeFile="activeFile"
          @select="handleSelectFile"
          @add-files="handleAddFiles"
          @toggle-favorite="handleToggleFavorite"
          @remove-file="handleRemoveFile"
          @open-settings="openSettings"
      />

      <PreviewCenter
          :file="activeFile"
          :pages="pages"
          :currentPageIndex="currentPageIndex"
          :selectedBlockIndex="selectedBlockIndex"
          :isParsing="isParsing"
          @select-block="handleSelectBlock"
          @change-page="handleChangePage"
          @update-block="handleUpdateBlock"
      />
    </div>

    <!-- ================= 系统设置弹窗 ================= -->
    <div v-if="showSettings" class="modal-mask" @click.self="closeSettings">
      <div class="modal-container">
        <div class="modal-header">
          <h3>系统设置</h3>
          <button class="close-btn" @click="closeSettings">✕</button>
        </div>

        <div class="modal-body">
          <div class="form-group">
            <label>API 服务地址 (Url)</label>
            <input type="text" v-model="config.url" placeholder="例如: http://localhost:8080" />
          </div>

          <div class="form-group">
            <label>模型名称 (ModelName)</label>
            <input type="text" v-model="config.modelName" placeholder="例如: paddle-ocr-v3" />
          </div>

          <div class="form-group">
            <label>API 密钥 (ApiKey)</label>
            <input type="password" v-model="config.apiKey" placeholder="请输入 API Key" />
          </div>

          <div v-if="saveMsg" class="save-msg" :class="{ success: saveMsg === '保存成功！' }">
            {{ saveMsg }}
          </div>
        </div>

        <div class="modal-footer">
          <button class="cancel-btn" @click="closeSettings">取消</button>
          <button class="save-btn" @click="saveSettings">保存设置</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import SidebarLeft from './components/SidebarLeft.vue'
import PreviewCenter from './components/PreviewCenter.vue'
import {
  ParseFile,
  GetConfig,
  UpdateConfig,
  SelectFile,
  GetRecentFiles,
  ToggleFavorite,
  RemoveRecentFile,
  GetParseResult,
  SaveParseResult
} from "../wailsjs/go/main/App"

export default {
  name: 'App',
  components: {
    SidebarLeft,
    PreviewCenter
  },

  data() {
    return {
      fileList: [],
      activeFile: null,
      pages: [],
      currentPageIndex: 0,
      selectedBlockIndex: -1,
      isParsing: false,
      ocrCache: {},

      showSettings: false,
      config: {
        url: '',
        modelName: '',
        apiKey: '',
      },
      saveMsg: ''
    }
  },

  async created() {
    await this.loadRecentFiles()
  },

  methods: {
    async loadRecentFiles() {
      try {
        const files = await GetRecentFiles()
        this.fileList = Array.isArray(files) ? files : []

        if (!this.activeFile && this.fileList.length > 0) {
          this.handleSelectFile(this.fileList[0])
        }
      } catch (e) {
        console.error('读取最近文件失败:', e)
      }
    },

    getCacheKey(file) {
      if (!file) return ''
      return file.path || file.id || ''
    },

    handleSelectFile(file) {
      this.activeFile = file
      this.selectedBlockIndex = -1
      this.currentPageIndex = 0

      if (!file) {
        this.pages = []
        return
      }

      const cacheKey = this.getCacheKey(file)
      if (cacheKey && this.ocrCache[cacheKey]) {
        this.pages = this.ocrCache[cacheKey]
        return
      }

      this.pages = []
      if (file.path) {
        this.loadParseResultOrOCR(file)
      }
    },

    async loadParseResultOrOCR(file) {
      const cacheKey = this.getCacheKey(file)

      this.isParsing = true
      this.pages = []

      try {
        const saved = await GetParseResult(file.path)
        if (Array.isArray(saved) && saved.length > 0) {
          this.pages = saved
          if (cacheKey) {
            this.ocrCache[cacheKey] = saved
          }
          this.isParsing = false
          return
        }
      } catch (e) {
        console.warn('读取本地识别结果失败，准备重新识别:', e)
      }

      this.runOCR(file)
    },

    handleAddFiles(files) {
      if (!Array.isArray(files) || files.length === 0) return

      this.fileList = files
      this.handleSelectFile(files[0])
    },

    async handleToggleFavorite(file) {
      try {
        const files = await ToggleFavorite(file.id || file.path)
        this.fileList = Array.isArray(files) ? files : []

        if (this.activeFile) {
          const updated = this.fileList.find(item =>
              item.id === this.activeFile.id || item.path === this.activeFile.path
          )
          if (updated) {
            this.activeFile = updated
          }
        }
      } catch (e) {
        console.error('切换收藏失败:', e)
      }
    },

    async handleRemoveFile(file) {
      try {
        const files = await RemoveRecentFile(file.id || file.path)
        this.fileList = Array.isArray(files) ? files : []

        const cacheKey = this.getCacheKey(file)
        if (cacheKey && this.ocrCache[cacheKey]) {
          delete this.ocrCache[cacheKey]
        }

        if (this.activeFile && (this.activeFile.id === file.id || this.activeFile.path === file.path)) {
          this.activeFile = null
          this.pages = []
          this.currentPageIndex = 0
          this.selectedBlockIndex = -1
        }
      } catch (e) {
        console.error('移除文件记录失败:', e)
      }
    },

    handleSelectBlock(index) {
      this.selectedBlockIndex = index
    },

    handleChangePage(pageIndex) {
      if (pageIndex < 0 || pageIndex >= this.pages.length) return
      this.currentPageIndex = pageIndex
      this.selectedBlockIndex = -1
    },

    handleUpdateBlock({ pageIndex, blockIndex, block }) {
      const newPages = JSON.parse(JSON.stringify(this.pages))
      newPages[pageIndex].blocks[blockIndex] = block
      this.pages = newPages

      const cacheKey = this.getCacheKey(this.activeFile)
      if (cacheKey) {
        this.ocrCache[cacheKey] = newPages
      }

      // 编辑后持久化，下次打开直接读取修改后的结果
      if (this.activeFile?.path) {
        SaveParseResult(this.activeFile.path, newPages)
            .catch(err => {
              console.error('保存识别结果失败:', err)
            })
      }
    },

    runOCR(file) {
      this.isParsing = true
      this.pages = []

      ParseFile(file.path)
          .then(result => {
            const pages = Array.isArray(result) ? result : []
            this.pages = pages

            const cacheKey = this.getCacheKey(file)
            if (cacheKey) {
              this.ocrCache[cacheKey] = pages
            }

            this.isParsing = false

            // 后端 ParseFile 里已经保存过一次，这里再兜底保存一次
            SaveParseResult(file.path, pages).catch(err => {
              console.warn('兜底保存识别结果失败:', err)
            })
          })
          .catch(err => {
            console.error('前端: 解析失败捕获到异常:', err)

            this.pages = [{
              pageIndex: 0,
              imagePath: '',
              blocks: [],
              error: err?.message ? String(err.message) : String(err)
            }]

            this.isParsing = false
          })
    },

    async openSettings() {
      try {
        const cfg = await GetConfig()
        this.config = cfg
        this.showSettings = true
        this.saveMsg = ''
      } catch (e) {
        console.error('获取配置失败:', e)
        this.saveMsg = '获取配置失败'
        this.showSettings = true
      }
    },

    async saveSettings() {
      this.saveMsg = '保存中...'
      try {
        await UpdateConfig(this.config)
        this.saveMsg = '保存成功！'

        setTimeout(() => {
          this.saveMsg = ''
          this.showSettings = false
        }, 1000)
      } catch (e) {
        console.error('保存失败:', e)
        this.saveMsg = '保存失败'
      }
    },

    closeSettings() {
      this.showSettings = false
      this.saveMsg = ''
    },

    async selectLibPath() {
      try {
        const path = await SelectFile("选择 Lib 路径")
        if (path) this.config.libPath = path
      } catch (e) {
        console.error('选择路径失败:', e)
      }
    },

    async selectLayoutPath() {
      try {
        const path = await SelectFile("选择 Layout 路径")
        if (path) this.config.layoutPath = path
      } catch (e) {
        console.error('选择路径失败:', e)
      }
    }
  }
}
</script>

<style scoped>
.app-root {
  height: 100vh;
  display: flex;
  flex-direction: column;
  background: #f0f2f5;
}

.workspace {
  flex: 1;
  display: flex;
  overflow: hidden;
}

.modal-mask {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.4);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
  backdrop-filter: blur(2px);
}

.modal-container {
  background: #fff;
  border-radius: 12px;
  width: 520px;
  max-width: calc(100vw - 40px);
  box-shadow: 0 10px 40px rgba(0, 0, 0, 0.15);
  overflow: hidden;
  animation: fadeIn 0.2s ease;
}

.modal-header {
  padding: 16px 20px;
  border-bottom: 1px solid #e8e8e8;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.modal-header h3 {
  margin: 0;
  font-size: 16px;
  color: #333;
}

.close-btn {
  background: none;
  border: none;
  font-size: 18px;
  color: #999;
  cursor: pointer;
}

.close-btn:hover {
  color: #333;
}

.modal-body {
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.form-group label {
  font-size: 13px;
  color: #666;
  font-weight: 500;
}

.form-group input {
  padding: 8px 12px;
  border: 1px solid #d9d9d9;
  border-radius: 6px;
  font-size: 13px;
  outline: none;
  transition: border-color 0.2s;
  width: 100%;
  box-sizing: border-box;
}

.form-group input:focus {
  border-color: #4f6ef7;
}

.input-with-btn {
  display: flex;
  gap: 8px;
}

.input-with-btn input {
  flex: 1;
}

.browse-btn {
  padding: 0 16px;
  background: #f5f5f5;
  border: 1px solid #d9d9d9;
  border-radius: 6px;
  cursor: pointer;
  font-size: 13px;
  color: #333;
  white-space: nowrap;
}

.browse-btn:hover {
  background: #e8e8e8;
}

.save-msg {
  font-size: 12px;
  color: #ff4d4f;
  text-align: center;
  margin-top: 8px;
}

.save-msg.success {
  color: #52c41a;
}

.modal-footer {
  padding: 12px 20px;
  border-top: 1px solid #e8e8e8;
  display: flex;
  justify-content: flex-end;
  gap: 12px;
}

.cancel-btn,
.save-btn {
  padding: 8px 20px;
  border-radius: 6px;
  font-size: 13px;
  cursor: pointer;
  border: 1px solid transparent;
  transition: all 0.2s;
}

.cancel-btn {
  background: #fff;
  border-color: #d9d9d9;
  color: #666;
}

.cancel-btn:hover {
  border-color: #4f6ef7;
  color: #4f6ef7;
}

.save-btn {
  background: #4f6ef7;
  color: #fff;
}

.save-btn:hover {
  background: #405be5;
}

@keyframes fadeIn {
  from {
    opacity: 0;
    transform: translateY(-10px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}
</style>