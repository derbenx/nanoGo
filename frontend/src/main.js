import './style.css';
import {
    GetImages,
    GetTasks,
    SelectAndAddMultipleImages,
    CreateNewImage,
    GetDefaultConfig,
    DeleteImage,
    AddTask,
    DeleteTask,
    RunTasks,
    RunBatch,
    SaveSessionUI,
    LoadSessionUI,
    GetConfig,
    SaveConfig,
    TestConnection,
    AddImages,
    GetImageBase64,
    ChangeImageUI,
    DuplicateTask,
    ToggleTaskDisabled,
    UpdateTask,
    GetCost,
    ResetCounters,
    GetLastGeneratedImage,
    HasGeneratedImage,
    ClearFinishedJobs,
    GetBatchJobs,
    OpenImageFolder,
    SendChatMessage,
    CalculateChatCost,
    ClearChatMemory,
    ClearTasks,
    ClearImages,
    DeleteSelectedImages,
    ProcessDroppedFiles
} from '../wailsjs/go/main/App';
import { EventsOn, OnFileDrop } from '../wailsjs/runtime/runtime';

let state = {
    images: [],
    tasks: [],
    config: {},
    activeTab: 'create',
    selectedImageID: null,
    selectedTaskID: null,
    isHoveringImage: false,
    isRunning: false,
    isBatchRunning: false
};

// --- Initialization ---

window.addEventListener('DOMContentLoaded', async () => {
    try {
        state.config = await GetConfig();
        populateSettings(state.config);

        OnFileDrop((x, y, paths) => {
            if (paths && paths.length > 0) {
                console.log("Files dropped:", paths);
                ProcessDroppedFiles(paths);
            }
        }, false);

        setupEventListeners();
        await refreshData();
        clearEditor();
        updateRatioPreview();
        renderAll();
        addLog("Application Initialized");
    } catch (err) {
        console.error("Initialization error:", err);
    }
});

async function refreshData() {
    state.images = await GetImages() || [];
    state.tasks = await GetTasks() || [];
    state.batchJobs = await GetBatchJobs() || [];

    if (state.selectedImageID && !state.images.find(i => i.ID == state.selectedImageID)) {
        state.selectedImageID = null;
    }
    if (state.selectedTaskID && !state.tasks.find(t => t.ID == state.selectedTaskID)) {
        state.selectedTaskID = null;
        clearEditor();
    }

    updateRunButtons();
}

function setupEventListeners() {
    EventsOn("log", (msg) => addLog(msg));
        EventsOn("run_started", () => {
        state.isRunning = true;
        updateRunButtons();
    });
    EventsOn("run_finished", () => {
        state.isRunning = false;
        updateRunButtons();
    });
    EventsOn("batch_run_started", () => {
        state.isBatchRunning = true;
        updateRunButtons();
    });
    EventsOn("batch_run_finished", () => {
        state.isBatchRunning = false;
        updateRunButtons();
    });
    EventsOn("images_updated", async () => {
        await refreshData();
        renderImageList();
        updateRunButtons();
    });
    EventsOn("tasks_updated", async () => {
        await refreshData();
        renderTaskList();
    });

    EventsOn("batch_updated", async () => {
        await refreshData();
        renderBatchList();
    });

    EventsOn("batch_timer", (seconds) => {
        const timerCont = document.getElementById('batch-timer-container');
        if (timerCont) timerCont.innerText = `Next check in: ${seconds}s`;
    });

    EventsOn("test_api_started", (mode) => {
        const status = document.getElementById('test-api-status-' + mode);
        if (status) {
            status.innerText = "Testing...";
            status.style.color = "#3498db";
        }
    });

    EventsOn("test_api_finished", (mode, success, msg) => {
        const status = document.getElementById('test-api-status-' + mode);
        if (status) {
            status.innerText = success ? "Success" : "Failed";
            status.style.color = success ? "#2ecc71" : "#e74c3c";
        }
    });

    document.querySelectorAll('.tab').forEach(tab => {
        tab.addEventListener('click', () => {
            document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
            tab.classList.add('active');
            state.activeTab = tab.dataset.tab;

            document.querySelectorAll('.tab-content').forEach(c => c.style.display = 'none');
            const target = document.getElementById('tab-' + state.activeTab);

            if (state.activeTab === 'create' || state.activeTab === 'chat') {
                target.style.display = 'flex';
            } else {
                target.style.display = 'block';
            }

            const logs = document.getElementById('logs');
            var hider='block'
            if (state.activeTab === 'chat' || state.activeTab === 'settings' || state.activeTab === 'help') { hider='none'; }
            logs.style.display = hider;

            renderAll();
        });
    });

    // Control Buttons
    document.getElementById('btn-load-session').onclick = () => LoadSessionUI();
    document.getElementById('btn-save-session').onclick = () => SaveSessionUI();
    document.getElementById('btn-run-immediate').onclick = (e) => {
        RunTasks().catch(err => {
            addLog("Error starting immediate tasks: " + err);
            updateRunButtons();
        });
        addLog("Triggering immediate execution...");
    };
    document.getElementById('btn-run-batch').onclick = () => {
        RunBatch().catch(err => {
            addLog("Error starting batch job: " + err);
            updateRunButtons();
        });
        addLog("Triggering batch submission...");
    };

    const chatInput = document.getElementById('chat-input');
    if (chatInput) {
        chatInput.oninput = async () => {
            const text = chatInput.value;
            const model = document.getElementById('chat-agent-select').value;
            const cost = await CalculateChatCost(model, text);
            document.getElementById('chat-cost-estimate').innerText = `Estimate: $${cost.toFixed(6)}`;
        };
    }

    // Global Exposure
    window.TogglePassword = (id) => {
        const input = document.getElementById(id);
        if (input) {
            input.type = input.type === 'password' ? 'text' : 'password';
        }
    };
    window.SelectAndAddMultipleImages = SelectAndAddMultipleImages;
    window.CreateNewImage = CreateNewImage;
    window.GetDefaultConfig = GetDefaultConfig;
    window.TestConnection = (mode) => TestConnection(mode);
    window.ToggleFreeMode = async (type, enabled) => {
        if (type === 'image') {
            state.config.is_free_mode_image = enabled;
        } else {
            state.config.is_free_mode_chat = enabled;
        }
        await SaveConfig(state.config);
        addLog(`Free Mode (${type}) ${enabled ? 'Enabled' : 'Disabled'}`);
    };
    window.UpdateChatConfig = async () => {
        state.config.chat_memory_enabled = document.getElementById('chat-memory-enabled').checked;
        state.config.chat_remember_initial = document.getElementById('chat-remember-initial').checked;
        state.config.chat_memory_slots = parseInt(document.getElementById('chat-memory-slots').value) || 3;
        state.config.chat_system_prompt = document.getElementById('chat-system-prompt').value;
        await SaveConfig(state.config);
        const input = document.getElementById('chat-input');
        if (input) {
            const agent = document.getElementById('chat-agent-select').value;
            const cost = await CalculateChatCost(agent, input.value);
            document.getElementById('chat-cost-estimate').innerText = `Estimate: $${cost.toFixed(6)}`;
        }
    };
    window.ResetToDefault = async (field) => {
        const def = await GetDefaultConfig();
        if (field === 'default_prompt') {
            document.getElementById('settings-default-prompt').value = def.default_prompt || '';
        } else if (field === 'default_neg_prompt') {
            document.getElementById('settings-default-neg-prompt').value = def.default_neg_prompt || '';
        } else if (field === 'encourage_gen') {
            document.getElementById('settings-encourage-gen').value = def.encourage_gen || '';
        } else if (field === 'encourage_edt') {
            document.getElementById('settings-encourage-edt').value = def.encourage_edt || '';
        } else if (field === 'temperature') {
            document.getElementById('settings-temp').value = def.temperature || 1.0;
        } else if (field === 'top_p') {
            document.getElementById('settings-top-p').value = def.top_p || 0.9;
        } else if (field === 'top_k') {
            document.getElementById('settings-top-k').value = def.top_k || 40;
        } else if (field === 'max_output_tokens') {
            document.getElementById('settings-max-tokens').value = def.max_output_tokens || 8192;
        } else if (field === 'safety_settings') {
            document.getElementById('safety-harassment').value = def.safety_settings[0].threshold;
            document.getElementById('safety-hate').value = def.safety_settings[1].threshold;
            document.getElementById('safety-sex').value = def.safety_settings[2].threshold;
            document.getElementById('safety-danger').value = def.safety_settings[3].threshold;
        } else if (field === 'models') {
            document.getElementById('settings-model-flash').value = def.model_nano_flash || '';
            document.getElementById('settings-model-pro').value = def.model_nano_pro || '';
            document.getElementById('settings-model-2').value = def.model_nano_2 || '';
            document.getElementById('settings-model-imagen').value = def.model_imagen || '';
            document.getElementById('settings-model-ultra').value = def.model_imagen_ultra || '';
        } else if (field === 'chat_model_list') {
            document.getElementById('settings-chat-models').value = def.chat_model_list || '';
        }
    };
    window.DeleteImage = DeleteImage;
    window.DeleteTask = (id) => {
        if (state.selectedTaskID === id) {
            state.selectedTaskID = null;
            clearEditor();
        }
        DeleteTask(id);
    };
    window.ChangeImageUI = ChangeImageUI;
    window.DuplicateTask = DuplicateTask;
    window.ToggleTaskDisabled = ToggleTaskDisabled;
    window.ResetCounters = ResetCounters;
    window.OpenImageFolder = OpenImageFolder;
    window.updateRatioPreview = updateRatioPreview;
    window.ClearTasks = ClearTasks;
    window.ClearImages = ClearImages;
    window.DeleteSelectedImages = DeleteSelectedImages;
    window.ClearChatMemory = async () => {
        await ClearChatMemory();
        document.getElementById('chat-history').innerHTML = '';
        addLog("Memory cleared.");
    };
    window.SendChatMessage = async () => {
        const input = document.getElementById('chat-input');
        const agent = document.getElementById('chat-agent-select').value;
        const history = document.getElementById('chat-history');
        const message = input.value.trim();
        if (!message) return;

        // Use safe message appending
        const userDiv = document.createElement('div');
        userDiv.style.marginBottom = '10px';
        userDiv.innerHTML = '<b>User:</b>';
        const userText = document.createElement('div');
        userText.textContent = message;
        userDiv.appendChild(userText);
        history.appendChild(userDiv);

        history.scrollTop = history.scrollHeight;
        const originalValue = input.value;
        input.value = '';
        document.getElementById('chat-cost-estimate').innerText = `Estimate: $0.000000`;

        try {
            const reply = await SendChatMessage(agent, message);
            const aiDiv = document.createElement('div');
            aiDiv.style.marginBottom = '10px';
            aiDiv.innerHTML = '<b>AI:</b>';
            const aiText = document.createElement('div');
            aiText.textContent = reply;
            aiDiv.appendChild(aiText);
            history.appendChild(aiDiv);
        } catch (err) {
            input.value = originalValue;
            const errDiv = document.createElement('div');
            errDiv.style.color = '#e74c3c';
            errDiv.innerHTML = '<b>Error:</b>';
            const errText = document.createElement('div');
            errText.textContent = err;
            errDiv.appendChild(errText);
            history.appendChild(errDiv);
        }
        history.scrollTop = history.scrollHeight;
    };
    window.ShowGeneratedImage = async (id) => {
        const b64 = await GetLastGeneratedImage(id);
        if (b64) {
            document.getElementById('editor-container').style.display = 'none';
            document.getElementById('preview-container').style.display = 'flex';
            const preview = document.getElementById('image-preview');
            preview.innerHTML = `<img src="data:image/jpeg;base64,${b64}" class="preview-image">`;
            state.isHoveringImage = true;
        } else {
            addLog("No generated image found for task " + id);
        }
    };
    window.ClearFinishedJobs = async () => {
        await ClearFinishedJobs();
        await refreshData();
        renderBatchList();
    };

    window.AddTaskFromUI = async () => {
        let selected = state.images.filter(img => img.Selected);
        if (selected.length === 0 && state.images.length === 1) {
            selected = [state.images[0]];
        }
        if (selected.length === 0 && state.selectedImageID) {
            const img = state.images.find(i => i.ID == state.selectedImageID);
            if (img) selected.push(img);
        }

        if (selected.length === 0) {
            addLog("Error: Select images (checkbox) first!");
            return;
        }

        const firstImg = selected[0];
        let ratio = document.getElementById('ratio-select').value;
        if (firstImg.Width > 0 && firstImg.Height > 0) {
            const targetRatio = firstImg.Width / firstImg.Height;
            const availableRatios = [
                { label: "1:8", val: 1 / 8 },
                { label: "1:4", val: 1 / 4 },
                { label: "9:16", val: 9 / 16 },
                { label: "2:3", val: 2 / 3 },
                { label: "3:4", val: 3 / 4 },
                { label: "4:5", val: 4 / 5 },
                { label: "1:1", val: 1 },
                { label: "5:4", val: 5 / 4 },
                { label: "4:3", val: 4 / 3 },
                { label: "3:2", val: 3 / 2 },
                { label: "16:9", val: 16 / 9 },
                { label: "21:9", val: 21 / 9 },
                { label: "4:1", val: 4 },
                { label: "8:1", val: 8 }
            ];

            let best = availableRatios[0];
            let minDiff = Math.abs(targetRatio - best.val);

            for (const r of availableRatios) {
                const diff = Math.abs(targetRatio - r.val);
                if (diff < minDiff) {
                    minDiff = diff;
                    best = r;
                }
            }
            ratio = best.label;
            document.getElementById('ratio-select').value = ratio;
            updateRatioPreview();
        }

        const imgIDs = selected.map(i => i.ID).join("+");
        const paths = selected.map(i => i.FullPath).join("|");
        const tier = document.getElementById('tier-select').value;
        const prompt = document.getElementById('prompt').value || state.config.default_prompt;
        const negPrompt = document.getElementById('neg-prompt').value || state.config.default_neg_prompt;

        const parts = tier.split(" ");
        const agent = parts.slice(0, -1).join(" ");
        const size = parts[parts.length - 1];
        const returnThought = document.getElementById('return-thought').checked;

        await AddTask(imgIDs, agent, size, ratio, prompt, negPrompt, paths, returnThought);
        addLog("Task added for: " + imgIDs);
    };

    let sourceFilterTimeout;
    window.UpdateTaskFromUI = async () => {
        if (!state.selectedTaskID) return;
        const task = state.tasks.find(t => t.ID === state.selectedTaskID);
        if (!task) return;

        const input = document.getElementById('source-ids');
        task.ImgIDs = input.value;

        clearTimeout(sourceFilterTimeout);
        sourceFilterTimeout = setTimeout(async () => {
            const currentIDs = state.images.map(img => img.ID);
            const parts = input.value.split('+').map(p => p.trim()).filter(p => p !== "");
            const filtered = parts.filter(p => currentIDs.includes(p));
            const newVal = filtered.join('+');
            if (newVal !== input.value) {
                input.value = newVal;
                task.ImgIDs = newVal;
                await UpdateTask(task);
            }
        }, 2000);
        task.Prompt = document.getElementById('prompt').value;
        task.ReturnThought = document.getElementById('return-thought').checked;
        task.NegativePrompt = document.getElementById('neg-prompt').value;
        const tier = document.getElementById('tier-select').value;
        const parts = tier.split(" ");
        task.Agent = parts.slice(0, -1).join(" ");
        task.Size = parts[parts.length - 1];

        filterRatios(task.Agent);
        task.Ratio = document.getElementById('ratio-select').value;

        await updateCostDisplay(task.Agent, task.Size);
        await UpdateTask(task);
        updateRatioPreview();
    };

    window.SaveSettings = async () => {
        const c = state.config;
        c.return_thought = document.getElementById('settings-return-thought').checked;
        c.api_key_paid = document.getElementById('settings-api-key-paid').value;
        c.api_key_free = document.getElementById('settings-api-key-free').value;
        c.output_dir = document.getElementById('settings-output-dir').value;
        c.debug = document.getElementById('settings-debug').checked;
        c.default_prompt = document.getElementById('settings-default-prompt').value;
        c.default_neg_prompt = document.getElementById('settings-default-neg-prompt').value;
        c.encourage_gen = document.getElementById('settings-encourage-gen').value;
        c.encourage_edt = document.getElementById('settings-encourage-edt').value;
        c.temperature = parseFloat(document.getElementById('settings-temp').value);
        c.top_p = parseFloat(document.getElementById('settings-top-p').value);
        c.top_k = parseInt(document.getElementById('settings-top-k').value);
        c.max_output_tokens = parseInt(document.getElementById('settings-max-tokens').value);

        c.safety_settings[0].threshold = document.getElementById('safety-harassment').value;
        c.safety_settings[1].threshold = document.getElementById('safety-hate').value;
        c.safety_settings[2].threshold = document.getElementById('safety-sex').value;
        c.safety_settings[3].threshold = document.getElementById('safety-danger').value;

        c.model_nano_flash = document.getElementById('settings-model-flash').value;
        c.model_nano_pro = document.getElementById('settings-model-pro').value;
        c.model_nano_2 = document.getElementById('settings-model-2').value;
        c.model_imagen = document.getElementById('settings-model-imagen').value;
        c.model_imagen_ultra = document.getElementById('settings-model-ultra').value;

        c.chat_model_list = document.getElementById('settings-chat-models').value;

        await SaveConfig(c);
        addLog("Configuration saved to config.json");
        const msg = document.getElementById('settings-saved-msg');
        if (msg) {
            msg.style.display = 'inline';
            setTimeout(() => {
                msg.style.display = 'none';
            }, 2000);
        }
    };


    // Vertical Resizer (Main)
    const mainResizer = document.getElementById('main-resizer');
    const leftPanel = document.getElementById('create-left');

    if (mainResizer && leftPanel) {
        let isResizing = false;

        mainResizer.addEventListener('mousedown', (e) => {
            isResizing = true;
            document.body.style.cursor = 'col-resize';
        });

        document.addEventListener('mousemove', (e) => {
            if (!isResizing) return;
            const containerWidth = document.querySelector('.main-content').offsetWidth;
            let offset = e.clientX / containerWidth;
            if (offset < 0.2) offset = 0.2;
            if (offset > 0.8) offset = 0.8;

            state.config.split_offset_main = offset;
            leftPanel.style.width = (offset * 100) + '%';
        });

        document.addEventListener('mouseup', async () => {
            if (isResizing) {
                isResizing = false;
                document.body.style.cursor = 'default';
                await SaveConfig(state.config);
            }
        });
    }

    // Global click to hide context menu
    document.addEventListener('click', () => {
        const menu = document.getElementById('context-menu');
        if (menu) menu.style.display = 'none';
    });

    // Disable custom context menu on inputs/textareas to allow native spellcheck/clipboard
    document.addEventListener('contextmenu', (e) => {
        if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA') {
            // Let native context menu through and stop propagation to prevent custom menus
            const menu = document.getElementById('context-menu');
            if (menu) menu.style.display = 'none';
            e.stopPropagation();
        }
    }, true);

    // Tab Cycling Shortcuts
    document.addEventListener('keydown', (e) => {
        if (e.ctrlKey && e.key === 'Tab') {
            e.preventDefault();
            const tabs = Array.from(document.querySelectorAll('.tab'));
            const activeIdx = tabs.findIndex(t => t.classList.contains('active'));
            let nextIdx;
            if (e.shiftKey) {
                nextIdx = (activeIdx - 1 + tabs.length) % tabs.length;
            } else {
                nextIdx = (activeIdx + 1) % tabs.length;
            }
            tabs[nextIdx].click();
        }
    });
}

function populateSettings(c) {
    if (!c) return;

    document.getElementById('settings-return-thought').checked = !!c.return_thought;

    if (c.split_offset_main) {
        const left = document.getElementById('create-left');
        if (left) left.style.width = (c.split_offset_main * 100) + '%';
    }
    document.getElementById('settings-api-key-paid').value = c.api_key_paid || '';
    document.getElementById('settings-api-key-free').value = c.api_key_free || '';

    document.getElementById('image-free-mode').checked = !!c.is_free_mode_image;
    document.getElementById('chat-free-mode').checked = !!c.is_free_mode_chat;

    document.getElementById('chat-memory-enabled').checked = !!c.chat_memory_enabled;
    document.getElementById('chat-remember-initial').checked = !!c.chat_remember_initial;
    document.getElementById('chat-memory-slots').value = c.chat_memory_slots || 3;
    const sysPrompt = document.getElementById('chat-system-prompt');
    if (sysPrompt) sysPrompt.value = c.chat_system_prompt || '';

    document.getElementById('settings-output-dir').value = c.output_dir || '';
    document.getElementById('settings-debug').checked = !!c.debug;
    document.getElementById('settings-default-prompt').value = c.default_prompt || '';
    document.getElementById('settings-default-neg-prompt').value = c.default_neg_prompt || '';
    document.getElementById('settings-encourage-gen').value = c.encourage_gen || '';
    document.getElementById('settings-encourage-edt').value = c.encourage_edt || '';
    document.getElementById('settings-temp').value = c.temperature || 1.0;
    document.getElementById('settings-top-p').value = c.top_p || 0.9;
    document.getElementById('settings-top-k').value = c.top_k || 40;
    document.getElementById('settings-max-tokens').value = c.max_output_tokens || 8192;

    if (c.safety_settings && c.safety_settings.length >= 4) {
        document.getElementById('safety-harassment').value = c.safety_settings[0].threshold;
        document.getElementById('safety-hate').value = c.safety_settings[1].threshold;
        document.getElementById('safety-sex').value = c.safety_settings[2].threshold;
        document.getElementById('safety-danger').value = c.safety_settings[3].threshold;
    }

    document.getElementById('settings-model-flash').value = c.model_nano_flash || '';
    document.getElementById('settings-model-pro').value = c.model_nano_pro || '';
    document.getElementById('settings-model-2').value = c.model_nano_2 || '';
    document.getElementById('settings-model-imagen').value = c.model_imagen || '';
    document.getElementById('settings-model-ultra').value = c.model_imagen_ultra || '';

    document.getElementById('settings-chat-models').value = c.chat_model_list || '';
}

function updateRunButtons() {
    const immBtn = document.getElementById('btn-run-immediate');
    const batchBtn = document.getElementById('btn-run-batch');

    if (!immBtn || !batchBtn) return;

    const allEnabledTasks = state.tasks.filter(t => !t.Disabled);
    const hasTasks = allEnabledTasks.length > 0;

    // Immediate mode tracking
    const runningImmediateTasks = state.tasks.filter(tk => (tk.RunningCount || 0) > 0);
    const immRunningCount = runningImmediateTasks.length;

    console.log(`[UI Update] isRunning (Imm): ${state.isRunning}, isBatchRunning: ${state.isBatchRunning}, immRunningTasks: ${immRunningCount}`);

    // Immediate button logic
    let immLabel = "RUN IMMEDIATE";
    let immDisabled = !hasTasks;

    if (allEnabledTasks.length === 1) {
        const t = allEnabledTasks[0];
        const taskRunningCount = t.RunningCount || 0;
        immLabel = `RUN IMMEDIATE (${taskRunningCount}/2)`;
        // If only one task, we allow up to 2 concurrent runs for it.
        immDisabled = (taskRunningCount >= 2);
    } else if (allEnabledTasks.length > 1) {
        // If multiple tasks, only one immediate task allowed at a time.
        if (state.isRunning) {
            immDisabled = true;
        }
    }

    const canImmediate = hasTasks && allEnabledTasks.every(t => {
        const hasPrompt = t.Prompt && t.Prompt.trim() !== "";
        const isImagen = t.Agent.includes("Imagen");
        const ids = (t.ImgIDs || "").split("+").map(id => id.trim()).filter(id => id !== "");
        const taskImages = state.images.filter(img => ids.includes(img.ID));
        const onlyGenerate = taskImages.length > 0 && taskImages.every(img => img.FullPath === "<GENERATE>");
        const noImages = ids.length === 0;
        if (isImagen && !(noImages || onlyGenerate)) return false;
        return hasPrompt;
    });
    immBtn.disabled = immDisabled || !canImmediate;
    immBtn.innerText = immLabel;

    // Batch button logic
    let canBatch = hasTasks;
    if (canBatch) {
        const firstAgent = allEnabledTasks[0].Agent;
        if (firstAgent.includes("Imagen")) {
            canBatch = false;
        } else {
            canBatch = allEnabledTasks.every(t => {
                const hasPrompt = t.Prompt && t.Prompt.trim() !== "";
                return t.Agent === firstAgent && hasPrompt;
            });
        }
    }
    batchBtn.disabled = !canBatch || state.isBatchRunning;
}

async function showPreview(id) {
    const img = state.images.find(i => i.ID == id);
    if (!img || img.FullPath === "" || img.FullPath === "<GENERATE>") return;

    state.isHoveringImage = true;
    document.getElementById('editor-container').style.display = 'none';
    document.getElementById('preview-container').style.display = 'flex';

    const preview = document.getElementById('image-preview');
    try {
        const b64 = await GetImageBase64(img.FullPath);
        if (b64) {
            preview.innerHTML = `<img src="data:image/jpeg;base64,${b64}" class="preview-image">`;
        } else {
            preview.innerText = 'Error loading image data';
        }
    } catch (err) {
        preview.innerText = 'Error: ' + err;
    }
}

function hidePreview() {
    state.isHoveringImage = false;
    if (!state.selectedImageID) {
        document.getElementById('editor-container').style.display = 'block';
        document.getElementById('preview-container').style.display = 'none';
    }
}

function renderAll() {
    if (state.activeTab === 'create') {
        renderImageList();
        renderTaskList();
    } else if (state.activeTab === 'batches') {
        renderBatchList();
    } else if (state.activeTab === 'chat') {
        renderChatAgentSelect();
    }
}

function renderChatAgentSelect() {
    const select = document.getElementById('chat-agent-select');
    if (!select) return;
    const currentVal = select.value;
    select.innerHTML = '';
    const models = (state.config.chat_model_list || "").split('\n').map(m => m.trim()).filter(m => m !== "");
    models.forEach(m => {
        const opt = document.createElement('option');
        opt.value = m;
        opt.innerText = m;
        select.appendChild(opt);
    });
    if (currentVal && models.includes(currentVal)) {
        select.value = currentVal;
    }
}

// --- Image List ---

function renderImageList() {
    const list = document.getElementById('image-list');
    if (!list) return;
    list.innerHTML = '';

    state.images.forEach(img => {
        const item = document.createElement('div');
        item.className = 'list-item' + (state.selectedImageID === img.ID ? ' selected' : '');
        item.innerHTML = `
            <span style="width: 30px"><input type="checkbox" ${img.Selected ? 'checked' : ''} class="img-check"></span>
            <span style="width: 30px">${img.ID}</span>
            <span style="width: 60px">${img.SizeMB.toFixed(2)}</span>
            <span style="width: 50px">${img.TaskCount}</span>
            <span style="flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;">${img.FileName}</span>
        `;

        const check = item.querySelector('.img-check');
        check.onchange = (e) => {
            img.Selected = e.target.checked;
            updateRunButtons();
        };

        item.onmouseenter = () => showPreview(img.ID);
        item.onmouseleave = () => hidePreview();

        item.onclick = (e) => {
            if (e.target.type !== 'checkbox') {
                state.selectedImageID = (state.selectedImageID === img.ID ? null : img.ID);
                state.selectedTaskID = null;
                clearEditor();
                renderImageList();
                renderTaskList();
            }
        };

        item.oncontextmenu = (e) => {
            e.preventDefault();
            const menu = [
                { label: 'Change Image', action: () => window.ChangeImageUI(img.ID) },
                { label: 'Delete Image', action: () => window.DeleteImage(img.ID) }
            ];
            const selected = state.images.filter(i => i.Selected);
            if (selected.length > 0) {
                menu.push({ label: `Delete Checked (${selected.length})`, action: () => window.DeleteSelectedImages() });
            }
            showContextMenu(e.clientX, e.clientY, menu);
        };

        list.appendChild(item);
    });
}

// --- Task List ---

function renderTaskList() {
    const list = document.getElementById('task-list');
    if (!list) return;
    list.innerHTML = '';

    state.tasks.forEach(task => {
        const item = document.createElement('div');
        item.className = 'list-item' + (state.selectedTaskID === task.ID ? ' selected' : '');
        if (task.Disabled) item.style.opacity = '0.5';

        item.innerHTML = `
            <span style="width: 60px">${task.ImgIDs}</span>
            <span style="width: 120px">${task.Agent} ${task.Size}</span>
            <span style="width: 60px">${task.Ratio}</span>
            <span style="width: 100px">${task.Status}</span>
            <span style="flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;">${task.Prompt}</span>
        `;

        item.onclick = () => {
            state.selectedTaskID = (state.selectedTaskID === task.ID ? null : task.ID);
            state.selectedImageID = null;

            if (state.selectedTaskID) {
                document.getElementById('editor-container').style.display = 'block';
                document.getElementById('preview-container').style.display = 'none';
                populateEditor(task);
            } else {
                clearEditor();
            }

            renderImageList();
            renderTaskList();
        };

        item.oncontextmenu = async (e) => {
            e.preventDefault();
            const hasImg = await HasGeneratedImage(task.ID);
            const menuItems = [
                { label: task.Disabled ? 'Enable' : 'Disable', action: () => window.ToggleTaskDisabled(task.ID) }
            ];
            if (hasImg) {
                menuItems.push({ label: 'Show Generated Image', action: () => window.ShowGeneratedImage(task.ID) });
            }
            menuItems.push({ label: 'Duplicate Task', action: () => window.DuplicateTask(task.ID) });
            menuItems.push({ label: 'Delete Task', action: () => window.DeleteTask(task.ID) });

            showContextMenu(e.clientX, e.clientY, menuItems);
        };

        list.appendChild(item);
    });
}

async function populateEditor(task) {
    document.getElementById('source-ids').value = task.ImgIDs;
    document.getElementById('prompt').value = task.Prompt;
    document.getElementById('return-thought').checked = !!task.ReturnThought;
    document.getElementById('neg-prompt').value = task.NegativePrompt;
    document.getElementById('tier-select').value = task.Agent + " " + task.Size;
    filterRatios(task.Agent);
    document.getElementById('ratio-select').value = task.Ratio;
    await updateCostDisplay(task.Agent, task.Size);
    updateRatioPreview();
}

async function clearEditor() {
    document.getElementById('source-ids').value = '';
    document.getElementById('prompt').value = '';
    document.getElementById('return-thought').checked = state.config.return_thought;
    document.getElementById('neg-prompt').value = '';
    document.getElementById('tier-select').selectedIndex = 0;
    const tier = document.getElementById('tier-select').value;
    filterRatios(tier);
    document.getElementById('ratio-select').selectedIndex = 0;
    await updateCostDisplay(tier.split(" ").slice(0, -1).join(" "), tier.split(" ").pop());
    updateRatioPreview();
}

function filterRatios(agent) {
    const isNano2 = agent.includes("Nano 2");
    const ratioSelect = document.getElementById('ratio-select');
    const options = ratioSelect.options;

    for (let i = 0; i < options.length; i++) {
        const val = options[i].value;
        if (val === "1:8" || val === "1:4" || val === "4:1" || val === "8:1") {
            options[i].style.display = isNano2 ? "block" : "none";
            if (!isNano2 && ratioSelect.value === val) {
                ratioSelect.value = "1:1";
            }
        }
    }
}

async function updateCostDisplay(agent, size) {
    const isImagen = agent.includes("Imagen");
    const costImm = await GetCost(agent, size, "Immediate");
    if (isImagen) {
        document.getElementById('cost-display').innerText = `Immediate: $${costImm.toFixed(4)}`;
    } else {
        const costBatch = await GetCost(agent, size, "Batch");
        document.getElementById('cost-display').innerText = `Immediate: $${costImm.toFixed(4)} | Batch: $${costBatch.toFixed(4)}`;
    }
}

// --- Batch List ---

function renderBatchList() {
    const list = document.getElementById('batch-list');
    if (!list) return;
    list.innerHTML = '';

    if (state.batchJobs.length === 0) {
        list.innerHTML = '<div style="padding: 20px; color: #888;">No active batch jobs.</div>';
        return;
    }

    state.batchJobs.forEach(job => {
        const item = document.createElement('div');
        item.className = 'batch-item';
        item.style = 'background-color: #151d29; border: 1px solid #34495e; padding: 15px; margin-bottom: 10px; border-radius: 8px;';

        const isFinished = ["SUCCEEDED", "FAILED", "CANCELLED", "EXPIRED", "Success", "Failed"].includes(job.Status);
        const progress = isFinished ? 100 : (job.Status === "Submitted" ? 10 : 50); // Dummy progress for now as API doesn't provide it clearly in percentage

        item.innerHTML = `
            <div style="display: flex; justify-content: space-between; margin-bottom: 8px;">
                <span style="font-weight: bold; color: #3498db;">${job.JobID}</span>
                <span style="color: ${isFinished ? (job.Status === 'SUCCEEDED' ? '#2ecc71' : '#e74c3c') : '#f1c40f'}">${job.Status}</span>
            </div>
            <div style="font-size: 0.85em; color: #888; margin-bottom: 10px;">Submitted at: ${new Date(job.SubmittedAt).toLocaleString()}</div>
            <div class="progress-bar-bg" style="background-color: #2c3e50; height: 10px; border-radius: 5px; overflow: hidden;">
                <div class="progress-bar-fill" style="background-color: #3498db; width: ${progress}%; height: 100%; transition: width 0.3s;"></div>
            </div>
        `;
        list.appendChild(item);
    });
}

// --- Context Menu ---

function showContextMenu(x, y, items) {
    let menu = document.getElementById('context-menu');
    if (!menu) {
        menu = document.createElement('div');
        menu.id = 'context-menu';
        menu.className = 'context-menu';
        document.body.appendChild(menu);
    }

    menu.innerHTML = '';
    items.forEach(item => {
        const div = document.createElement('div');
        div.innerText = item.label;
        div.onclick = item.action;
        menu.appendChild(div);
    });

    menu.style.left = x + 'px';
    menu.style.top = y + 'px';
    menu.style.display = 'flex';
}

// --- Logs ---

function updateRatioPreview() {
    const canvas = document.getElementById('ratio-canvas');
    if (!canvas) return;
    const ctx = canvas.getContext('2d');
    const ratioStr = document.getElementById('ratio-select').value;
    const parts = ratioStr.split(':');
    const rw = parseInt(parts[0]);
    const rh = parseInt(parts[1]);

    const cw = canvas.width;
    const ch = canvas.height;
    ctx.clearRect(0, 0, cw, ch);

    // Margin for the box
    const margin = 10;
    const availW = cw - margin * 2;
    const availH = ch - margin * 2;

    let targetW, targetH;
    if (rw / rh > availW / availH) {
        targetW = availW;
        targetH = availW * (rh / rw);
    } else {
        targetH = availH;
        targetW = availH * (rw / rh);
    }

    const x = (cw - targetW) / 2;
    const y = (ch - targetH) / 2;

    ctx.fillStyle = '#3498db';
    ctx.globalAlpha = 0.3;
    ctx.fillRect(x, y, targetW, targetH);

    ctx.strokeStyle = '#3498db';
    ctx.globalAlpha = 1.0;
    ctx.lineWidth = 2;
    ctx.strokeRect(x, y, targetW, targetH);

    // Text label inside
    ctx.fillStyle = 'white';
    ctx.font = '10px Arial';
    ctx.textAlign = 'center';
    ctx.fillText(ratioStr, cw / 2, ch / 2 + 4);
}

function addLog(msg) {
    const logArea = document.getElementById('logs');
    if (!logArea) return;
    const entry = document.createElement('div');
    const time = new Date().toLocaleTimeString();
    entry.innerText = `[${time}] ${msg}`;
    logArea.appendChild(entry);
    logArea.scrollTop = logArea.scrollHeight;
}
