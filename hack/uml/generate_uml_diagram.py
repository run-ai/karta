#!/usr/bin/env python3
"""
UML Diagram Generator for RID Architecture

This script generates a PNG diagram from the PlantUML source file.
It uses a virtual environment for dependency isolation and provides clear error messages.

Usage: Run from the repository root:
  python3 hack/uml/generate_uml_diagram.py
"""

import subprocess
import sys
import os
from pathlib import Path

def run_command(cmd, description):
    """Run a shell command and return success status"""
    print(f"-> {description}")
    try:
        result = subprocess.run(cmd, shell=True, capture_output=True, text=True)
        if result.returncode == 0:
            print(f"   Success")
            if result.stdout and result.stdout.strip():
                print(f"   Output: {result.stdout.strip()}")
            return True
        else:
            print(f"   Failed (exit code {result.returncode})")
            if result.stderr and result.stderr.strip():
                print(f"   Error: {result.stderr.strip()}")
            return False
    except Exception as e:
        print(f"   Exception: {e}")
        return False

def setup_virtual_environment():
    """Create and setup virtual environment with required packages"""
    venv_path = Path('hack/uml/venv')
    
    print("\nSetting up virtual environment...")
    
    # Create virtual environment if it doesn't exist
    if not venv_path.exists():
        if not run_command(
            f"python3 -m venv {venv_path}",
            "Creating virtual environment"
        ):
            return False
    else:
        print("   Virtual environment already exists")
    
    # Determine pip path
    if sys.platform == "win32":
        pip_path = venv_path / "Scripts" / "pip"
        python_path = venv_path / "Scripts" / "python"
    else:
        pip_path = venv_path / "bin" / "pip"
        python_path = venv_path / "bin" / "python"
    
    # Install required packages
    required_packages = ['plantuml', 'six', 'httplib2']
    
    for package in required_packages:
        if not run_command(
            f"{pip_path} install {package}",
            f"Installing {package} in virtual environment"
        ):
            print(f"   Warning: Failed to install {package}")
    
    return python_path

def check_module_in_venv(python_path, module_name):
    """Check if a module is available in the virtual environment"""
    try:
        result = subprocess.run(
            f"{python_path} -c 'import {module_name}'",
            shell=True,
            capture_output=True,
            text=True
        )
        return result.returncode == 0
    except Exception:
        return False

def generate_diagram_with_plantuml(python_path):
    """Generate diagram using Python plantuml module in virtual environment"""
    print("\nGenerating UML diagram using Python plantuml module...")
    
    # Create the generation script content
    generation_script = '''
import plantuml
from pathlib import Path

try:
    # Create PlantUML server instance
    server = plantuml.PlantUML(url='http://www.plantuml.com/plantuml/img/')
    
    # Read PlantUML source
    puml_file = Path('hack/uml/rid-uml-diagram.puml')
    with open(puml_file, 'r') as f:
        puml_code = f.read()
    
    # Generate PNG
    output_file = Path('docs/design/rid-architecture.png')
    output_file.parent.mkdir(parents=True, exist_ok=True)
    
    with open(output_file, 'wb') as f:
        png_data = server.processes(puml_code)
        f.write(png_data)
    
    print(f"Generated UML diagram: {output_file}")
    print(f"File size: {output_file.stat().st_size} bytes")
    
except Exception as e:
    print(f"Error: {e}")
    exit(1)
'''
    
    # Write temporary script
    temp_script = Path('hack/uml/temp_generate.py')
    with open(temp_script, 'w') as f:
        f.write(generation_script)
    
    try:
        # Run the script with the virtual environment Python
        result = run_command(
            f"{python_path} {temp_script}",
            "Running diagram generation"
        )
        return result
    finally:
        # Clean up temporary script
        if temp_script.exists():
            temp_script.unlink()

def generate_diagram_with_jar():
    """Alternative: Generate diagram using PlantUML JAR (if available)"""
    print("\nAttempting to use PlantUML JAR (alternative method)...")
    
    # Check if Java is available
    if not run_command("java -version", "Checking Java availability"):
        print("   Java not available - cannot use PlantUML JAR")
        return False
    
    # Try to download PlantUML JAR if not present
    jar_file = Path('hack/uml/plantuml.jar')
    if not jar_file.exists():
        print("   Downloading PlantUML JAR...")
        if not run_command(
            f"curl -L -o {jar_file} http://sourceforge.net/projects/plantuml/files/plantuml.jar/download",
            "Downloading PlantUML JAR"
        ):
            return False
    
    # Ensure output directory exists
    Path('docs/design').mkdir(parents=True, exist_ok=True)
    
    # Generate diagram
    return run_command(
        f"java -jar {jar_file} -tpng -o docs/design hack/uml/rid-uml-diagram.puml",
        "Generating PNG with PlantUML JAR"
    )

def main():
    """Main execution function"""
    print("RID Architecture UML Diagram Generator")
    print("=" * 50)
    
    # Check current directory (should be repository root)
    if not Path('hack/uml/rid-uml-diagram.puml').exists():
        print("Error: Please run this script from the repository root")
        print("   Expected file: hack/uml/rid-uml-diagram.puml")
        print("   Usage: python3 hack/uml/generate_uml_diagram.py")
        return 1
    
    print("Working directory looks correct")
    
    # Method 1: Try Python plantuml module in virtual environment
    python_path = setup_virtual_environment()
    if not python_path:
        print("Failed to setup virtual environment")
    else:
        # Check if required modules are available
        modules_available = all(
            check_module_in_venv(python_path, module)
            for module in ['plantuml', 'six']
        )
        
        if modules_available:
            if generate_diagram_with_plantuml(python_path):
                print("\nSuccess! UML diagram generated successfully.")
                print("Output location: docs/design/rid-architecture.png")
                return 0
            else:
                print("\nPython method failed, trying alternative...")
        else:
            print("Required modules not available in virtual environment")
    
    # Method 2: Try PlantUML JAR as fallback
    if generate_diagram_with_jar():
        print("\nSuccess! UML diagram generated using PlantUML JAR.")
        print("Output location: docs/design/rid-architecture.png")
        return 0
    
    # If all methods failed
    print("\nAll diagram generation methods failed.")
    print("\nManual alternatives:")
    print("   1. Visit: http://www.plantuml.com/plantuml/uml/")
    print("   2. Copy content from: hack/uml/rid-uml-diagram.puml")
    print("   3. Generate diagram online and save as docs/design/rid-architecture.png")
    print("   4. Install PlantUML locally: brew install plantuml (macOS)")
    
    return 1

if __name__ == "__main__":
    exit(main()) 