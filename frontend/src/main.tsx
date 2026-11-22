import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import {
  CellStyleModule, ClientSideRowModelModule, ModuleRegistry, NumberFilterModule, QuickFilterModule, RowSelectionModule,
  TextFilterModule, TooltipModule, ValidationModule,
} from 'ag-grid-community'
import '@fontsource-variable/roboto-flex'
import 'material-symbols/outlined.css'
import './styles/tokens.css'
import './styles/app.css'
import App from './App'

// Only the grid features the app uses (AllCommunityModule pulled in editors,
// CSV export, pagination…). ValidationModule reports a missing module in dev.
ModuleRegistry.registerModules([
  ClientSideRowModelModule, CellStyleModule, TextFilterModule, NumberFilterModule, QuickFilterModule, RowSelectionModule, TooltipModule,
  ...(import.meta.env.DEV ? [ValidationModule] : []),
])

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
