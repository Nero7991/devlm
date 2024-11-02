# MIT License
# 
# Copyright (c) 2024 Oren Collaco
# 
# Permission is hereby granted, free of charge, to any person obtaining a copy
# of this software and associated documentation files (the "Software"), to deal
# in the Software without restriction, including without limitation the rights
# to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
# copies of the Software, and to permit persons to whom the Software is
# furnished to do so, subject to the following conditions:
# 
# The above copyright notice and this permission notice shall be included in all
# copies or substantial portions of the Software.
# 
# THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
# IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
# FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
# AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
# LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
# OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
# SOFTWARE.

import unittest

import sys
sys.path.append('..') 
from bootstrap import parse_modification_commands, apply_modifications, process_file_modifications

class TestFileModifications(unittest.TestCase):
    def setUp(self):
        self.sample_content = """def hello():
    print("Hello")
    
def world():
    print("World")"""

    def test_single_add_command(self):
        llm_response = """ADD 2:<CONTENT_START>    print("How are you?")
    print("I am fine")<CONTENT_END>"""
        commands, error = parse_modification_commands(llm_response)
        self.assertIsNone(error)
        self.assertEqual(len(commands), 1)
        self.assertEqual(commands[0][0], 'ADD')
        
        modified, summary, error = apply_modifications(self.sample_content, commands)
        self.assertIn('print("How are you?")', modified)
        self.assertIn('Added after line 2:', summary)

    def test_multiple_add_commands(self):
        llm_response = """ADD 1:<CONTENT_START>    print("First")<CONTENT_END>
ADD 3:<CONTENT_START>    print("Second")<CONTENT_END>"""
        commands, error = parse_modification_commands(llm_response)
        self.assertIsNone(error)
        self.assertEqual(len(commands), 2)
        
        modified, summary, error = apply_modifications(self.sample_content, commands)
        self.assertIn('print("First")', modified)
        self.assertIn('print("Second")', modified)

    def test_single_remove_command(self):
        llm_response = "REMOVE 2-3"
        commands, error = parse_modification_commands(llm_response)
        self.assertIsNone(error)
        self.assertEqual(len(commands), 1)
        
        modified, summary, error = apply_modifications(self.sample_content, commands)
        self.assertNotIn('print("Hello")', modified)
        self.assertIn('Removed lines 2-3:', summary)

    def test_single_remove_command_with_single_line_number(self):
        llm_response = "REMOVE 2"
        commands, error = parse_modification_commands(llm_response)
        self.assertIsNone(error)
        self.assertEqual(len(commands), 1)
        
        modified, summary, error = apply_modifications(self.sample_content, commands)
        self.assertNotIn('print("Hello")', modified)
        self.assertIn('Removed lines 2-2:', summary)

    def test_multiple_remove_commands(self):
        llm_response = """REMOVE 1-2
REMOVE 4-5"""
        commands, error = parse_modification_commands(llm_response)
        self.assertIsNone(error)
        self.assertEqual(len(commands), 2)
        
        modified, summary, error = apply_modifications(self.sample_content, commands)
        self.assertNotIn('print("Hello")', modified)
        self.assertNotIn('print("World")', modified)

    def test_single_modify_command(self):
        llm_response = """MODIFY 2-2:<CONTENT_START>    print("Modified Hello")<CONTENT_END>"""
        commands, error = parse_modification_commands(llm_response)
        self.assertIsNone(error)
        self.assertEqual(len(commands), 1)
        
        modified, summary, error = apply_modifications(self.sample_content, commands)
        self.assertIn('print("Modified Hello")', modified)
        self.assertIn('Modified lines 2-2:', summary)

    def test_multiple_modify_commands(self):
        llm_response = """MODIFY 2-2:<CONTENT_START>    print("Modified Hello")<CONTENT_END>
MODIFY 5-5:<CONTENT_START>    print("Modified World")<CONTENT_END>"""
        commands, error = parse_modification_commands(llm_response)
        self.assertIsNone(error)
        self.assertEqual(len(commands), 2)
        
        modified, summary, error = apply_modifications(self.sample_content, commands)
        self.assertIn('print("Modified Hello")', modified)
        self.assertIn('print("Modified World")', modified)

    def test_multiple_modify_with_single_line_number_commands(self):
        llm_response = """MODIFY 2:<CONTENT_START>    print("Modified Hello")<CONTENT_END>
MODIFY 5-5:<CONTENT_START>    print("Modified World")<CONTENT_END>"""
        commands, error = parse_modification_commands(llm_response)
        self.assertIsNone(error)
        self.assertEqual(len(commands), 2)
        
        modified, summary, error = apply_modifications(self.sample_content, commands)
        self.assertIn('print("Modified Hello")', modified)
        self.assertIn('print("Modified World")', modified)

    def test_mixed_commands(self):
        llm_response = """ADD 1:<CONTENT_START>    print("First")<CONTENT_END>
REMOVE 3-4"""
        commands, error = parse_modification_commands(llm_response)
        self.assertIsNotNone(error)
        self.assertIn("Cannot mix different command types", error)

    def test_invalid_line_numbers(self):
        llm_response = "REMOVE 999-1000"
        commands, error = parse_modification_commands(llm_response)
        self.assertIsNone(error)
        
        modified, summary, error = apply_modifications(self.sample_content, commands)
        self.assertIn("Warning: Could not remove lines 999-1000", summary)
        self.assertEqual(modified, self.sample_content)

    def test_multiline_content(self):
        llm_response = """ADD 1:<CONTENT_START>def setup():
    print("Setting up")
    return True<CONTENT_END>"""
        commands, error = parse_modification_commands(llm_response)
        self.assertIsNone(error)
        self.assertEqual(len(commands), 1)
        
        modified, summary, error = apply_modifications(self.sample_content, commands)
        self.assertIn('def setup():', modified)
        self.assertIn('print("Setting up")', modified)
        self.assertIn('return True', modified)

    def test_process_file_modifications(self):
        # Test the high-level function
        llm_response = """ADD 1:<CONTENT_START>    print("New line")<CONTENT_END>"""
        modified, summary2, summary = process_file_modifications(self.sample_content, llm_response)
        self.assertIn('print("New line")', modified)
        self.assertIn('Added after line 1:', summary2)

    def test_empty_content(self):
        llm_response = ""
        modified, summary2, summary = process_file_modifications("", llm_response)
        self.assertEqual(summary, "Error: No valid modification commands found.")
        self.assertEqual(modified, "")

    def test_malformed_commands(self):
        # Missing CONTENT_START
        llm_response = "ADD 1:print('test')<CONTENT_END>"
        modified, summary2, summary = process_file_modifications(self.sample_content, llm_response)
        self.assertEqual(modified, self.sample_content)
        
        # Missing CONTENT_END
        llm_response = "ADD 1:<CONTENT_START>print('test')"
        modified, summary2, summary = process_file_modifications(self.sample_content, llm_response)
        self.assertEqual(modified, self.sample_content)

    def test_line_number_adjustments(self):
        # Test that line numbers are properly adjusted after modifications
        llm_response = """ADD 1:<CONTENT_START>Line 1
Line 2<CONTENT_END>
ADD 2:<CONTENT_START>Line 3<CONTENT_END>"""
        commands, error = parse_modification_commands(llm_response)
        self.assertIsNone(error)
        
        modified, summary, error = apply_modifications("Original\nOriginal Line 1\nOriginal Line 2", commands)
        lines = modified.split('\n')
        self.assertEqual(lines[0], "Original")
        self.assertEqual(lines[1], "Line 1")
        self.assertEqual(lines[2], "Line 2")
        self.assertEqual(lines[3], "Original Line 1")
        self.assertEqual(lines[4], "Line 3")

    def test_invalid_command_format(self):
        llm_response = "INVALID 1-2"
        modified, summary2, summary = process_file_modifications(self.sample_content, llm_response)
        self.assertEqual(modified, self.sample_content)
        self.assertIn("Error: No valid modification commands found.", summary)

    def test_commands_with_surrounding_text(self):
        """Test parsing commands that are embedded within explanatory text"""
        llm_response = """Based on the inspection results and the current file content, I'll modify the file to fix the indentation issues according to PEP 8 standards. The main problems are:
1. The print statement inside hello_world() function needs proper indentation (4 spaces)
2. The if block needs consistent indentation

I'll use the MODIFY keyword to fix both indentation issues while maintaining the functionality:

MODIFY 1-4:<CONTENT_START>def hello_world():
    print("Hello World")

if __name__ == "__main__":
    hello_world()<CONTENT_END>

The changes include:
- Added 4 spaces before the print statement (line 2)
- Added a blank line between function definition and if statement for better readability (PEP 8)
- Added 4 spaces before hello_world() call inside the if block
These changes maintain the functionality while improving code style and readability according to Python standards.
        """
        test_input = """def hello_world():
print("Hello World")
if __name__ == "__main__":
hello_world()"""

        expected_output = """def hello_world():
    print("Hello World")

if __name__ == "__main__":
    hello_world()"""

        commands, error = parse_modification_commands(llm_response)
        self.assertIsNone(error)
        self.assertEqual(len(commands), 1)
        self.assertEqual(commands[0][0], 'MODIFY')
        # self.assertEqual(commands[0][1], 2)
        # self.assertEqual(commands[0][2], 2)
        #self.assertEqual(commands[0][3], '                    print("New line")')

        modified, summary2, summary = process_file_modifications(test_input, llm_response)
        print("Modified:")
        print(repr(modified))
        print("Expected:")
        print(repr(expected_output))
        self.assertEqual(modified, expected_output)
        self.assertIn('Modified lines 1-4:', summary2)

def test_addition_with_surrounding_text_and_newlines(self):
    """Test parsing ADD commands that are embedded within explanatory text and contain multiple newlines"""
    llm_response = """I'll add a docstring to the function to better document its purpose:
    
ADD 1:<CONTENT_START>def hello_world():
    '''
    A simple function that prints Hello World.
    
    This function demonstrates basic Python syntax
    and serves as a starting point for learning Python.
    '''

    print("Hello World")<CONTENT_END>

This addition:
- Adds clear documentation
- Follows PEP 257 docstring conventions
- Includes multiple blank lines for readability"""

    test_input = """def hello_world():
    print("Hello World")"""

    expected_output = """def hello_world():
    '''
    A simple function that prints Hello World.
    
    This function demonstrates basic Python syntax
    and serves as a starting point for learning Python.
    '''

    print("Hello World")"""

    commands, error = parse_modification_commands(llm_response)
    self.assertIsNone(error)
    self.assertEqual(len(commands), 1)
    self.assertEqual(commands[0][0], 'ADD')
    self.assertEqual(commands[0][1], 1)
    self.assertEqual(commands[0][2], 1)

    modified, summary = apply_modifications(test_input, commands)
    self.assertEqual(modified, expected_output)
    self.assertIn('Added after line 1:', summary)

if __name__ == '__main__':
    unittest.main(verbosity=2)