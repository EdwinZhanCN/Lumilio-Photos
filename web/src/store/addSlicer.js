// This is a test slicer for adding two numbers
import { createSlice } from '@reduxjs/toolkit';
import { createAsyncThunk } from '@reduxjs/toolkit';
import { configureStore } from '@reduxjs/toolkit';


// Async thunk to simulate an API call
export const addAsync = createAsyncThunk(
  'add/addAsync',
  async (numbers) => {
    return new Promise((resolve) => {
      setTimeout(() => {
        resolve(numbers);
      }, 1000);
    });
  }
);

// Create a slice
const addSlice = createSlice({
  name: 'add',
  initialState: {
    value: 0,
    status: 'idle',
  },
  reducers: {
    add: (state, action) => {
      state.value += action.payload;
    },
    reset: (state) => {
      state.value = 0;
    },
  },
  extraReducers: (builder) => {
    builder
      .addCase(addAsync.pending, (state) => {
        state.status = 'loading';
      })
      .addCase(addAsync.fulfilled, (state, action) => {
        state.status = 'succeeded';
        state.value += action.payload;
      })
      .addCase(addAsync.rejected, (state) => {
        state.status = 'failed';
      });
  },
});


export default addSlice.reducer;