import { BrowserRouter } from "react-router-dom";
import { DemoProvider } from "./contexts/DemoContext";
import { AuthProvider } from "./contexts/AuthContext";
import { AppRoutes } from "./routes/AppRoutes";

function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <DemoProvider>
          <AppRoutes />
        </DemoProvider>
      </AuthProvider>
    </BrowserRouter>
  );
}

export default App;
